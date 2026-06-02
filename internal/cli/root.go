package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/soriano/nota/internal/domain"
	"github.com/soriano/nota/internal/infra/editor"
	"github.com/soriano/nota/internal/infra/ollama"
	"github.com/soriano/nota/internal/infra/sqlite"
	"github.com/soriano/nota/internal/server"
	"github.com/soriano/nota/internal/usecase"
)

var (
	tagsFlag    []string
	notebookFlag string
	catFlag     string
	contentFlag string
	sortFlag    string
	limitFlag   int
	jsonFlag    bool
	rawFlag     bool
	forceFlag   bool
)

var Version = "dev"

var defaultStoragePath string

func init() {
	home, _ := os.UserHomeDir()
	defaultStoragePath = filepath.Join(home, ".nota")
}

type App struct {
	docRepo   domain.DocumentRepository
	linkRepo  domain.DocumentLinkRepository
	config    *domain.Config
	embed     domain.EmbeddingService
	docDB     *sqlite.DocumentRepo
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "nota",
		Short:        "CLI Knowledge Base powered by markdown",
		Long:         "Nota is a CLI tool for storing and searching markdown notes with semantic search.",
		Version:      Version,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringSliceVarP(&tagsFlag, "tags", "t", nil, "tags (comma-separated)")
	root.PersistentFlags().StringVarP(&notebookFlag, "notebook", "b", "", "notebook")
	root.PersistentFlags().StringVarP(&catFlag, "cat", "c", "", "category")

	root.AddCommand(newCmd())
	root.AddCommand(saveCmd())
	root.AddCommand(editCmd())
	root.AddCommand(openCmd())
	root.AddCommand(deleteCmd())
	root.AddCommand(listCmd())
	root.AddCommand(searchCmd())
	root.AddCommand(linkCmd())
	root.AddCommand(tagsCmd())
	root.AddCommand(backupCmd())
	root.AddCommand(restoreCmd())
	root.AddCommand(cleanCmd())
	root.AddCommand(configCmd())
	root.AddCommand(setupCmd())
	root.AddCommand(serveCmd())

	return root
}

func ensureSetup() (*App, error) {
	ctx := context.Background()
	configPath := filepath.Join(defaultStoragePath, "config.json")
	dbPath := filepath.Join(defaultStoragePath, "data.db")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		runSetup(ctx, defaultStoragePath)
	}

	cfgRepo := sqlite.NewConfigRepository(defaultStoragePath)
	cfg, err := cfgRepo.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	docRepo, err := sqlite.NewDocumentRepository(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	linkRepo := sqlite.NewLinkRepository(docRepo.DB())
	embedSvc := ollama.New(cfg.OllamaURL, cfg.OllamaModel)

	return &App{
		docRepo:  docRepo,
		linkRepo: linkRepo,
		config:   cfg,
		embed:    embedSvc,
		docDB:    docRepo,
	}, nil
}

func newCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "new", Short: "Create a new note", Aliases: []string{"n"}}
	cmd.Flags().StringVar(&contentFlag, "content", "", "content (skip editor)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewCreateUseCase(app.docRepo, app.embed, app.config.Editor)
		doc, err := uc.Execute(context.Background(), usecase.CreateInput{
			Tags:      tagsFlag,
			Notebook:  notebookFlag,
			Category:  catFlag,
			Content:   contentFlag,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Created: %s (%s)\n", doc.Title, doc.ID)
		return nil
	}
	return cmd
}

func saveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save [text]",
		Short: "Quick capture a note",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}

			var content string
			if len(args) > 0 {
				content = args[0]
			}

			stat, _ := os.Stdin.Stat()
			fromPipe := stat.Mode()&os.ModeCharDevice == 0
			if fromPipe {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading pipe: %w", err)
				}
				content = string(b)
			}

			if content == "" {
				return fmt.Errorf("provide text argument or pipe content: nota save \"text\" or echo \"text\" | nota save")
			}

			uc := usecase.NewSaveUseCase(app.docRepo, app.embed)
			doc, err := uc.Execute(context.Background(), usecase.SaveInput{
				Content:   content,
				Tags:      tagsFlag,
				Notebook:  notebookFlag,
				Category:  catFlag,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Saved: %s (%s)\n", doc.Title, doc.ID)
			return nil
		},
	}
}

func editCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit [id]",
		Short:   "Edit a note",
		Aliases: []string{"e"},
	}
	cmd.Flags().StringVar(&contentFlag, "content", "", "new content (skip editor, for agents)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		ctx := context.Background()

		if contentFlag != "" && len(args) > 0 {
			doc, err := app.docRepo.GetByID(ctx, args[0])
			if err != nil {
				return fmt.Errorf("document not found: %w", err)
			}
			doc.Content = contentFlag
			doc.Title = doc.ExtractTitle()
			embedding, err := app.embed.Generate(ctx, contentFlag)
			if err == nil {
				doc.Embedding = embedding
			}
			if err := app.docRepo.Update(ctx, doc); err != nil {
				return err
			}
			fmt.Printf("Updated: %s (%s)\n", doc.Title, doc.ID)
			return nil
		}

		if len(args) > 0 {
			uc := usecase.NewEditUseCase(app.docRepo, app.embed, app.config.Editor)
			if err := uc.Execute(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Updated: %s\n", args[0])
			return nil
		}

		return fmt.Errorf("provide an id: nota edit <id> --content \"...\" or use nota serve")
	}
	return cmd
}

func openCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "open [id]",
		Short:   "Open and view a note",
		Aliases: []string{"o"},
	}
	cmd.Flags().BoolVar(&rawFlag, "raw", false, "output raw markdown")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("provide an id: nota open <id> --raw or use nota serve")
		}
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewOpenUseCase(app.docRepo)
		doc, err := uc.Execute(context.Background(), args[0])
		if err != nil {
			return err
		}
		fmt.Println(doc.Content)
		return nil
	}
	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [id]",
		Short:   "Delete a note",
		Aliases: []string{"d"},
	}
	cmd.Flags().BoolVar(&forceFlag, "force", false, "skip confirmation")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("provide an id: nota delete <id> --force or use nota serve")
		}
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewDeleteUseCase(app.docRepo)
		return uc.Execute(context.Background(), args[0])
	}
	return cmd
}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List notes",
		Aliases: []string{"ls"},
	}
	cmd.Flags().StringVar(&sortFlag, "sort", "recent", "sort order: recent, accessed, alpha")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&rawFlag, "raw", false, "output plain text table")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewListUseCase(app.docRepo)
		docs, err := uc.Execute(context.Background(), usecase.ListInput{
			Tags:      tagsFlag,
			Notebook:  notebookFlag,
			Category:  catFlag,
			Sort:      sortFlag,
			Limit:     40,
		})
		if err != nil {
			return err
		}
		if jsonFlag {
			data, _ := json.MarshalIndent(docs, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		if len(docs) == 0 {
			fmt.Println("No notes found")
			return nil
		}
		printDocTable(docs)
		return nil
	}
	return cmd
}

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search [query]",
		Short:   "Semantic search notes",
		Aliases: []string{"s"},
		Args:    cobra.MinimumNArgs(1),
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&rawFlag, "raw", false, "output plain text table")
	cmd.Flags().IntVar(&limitFlag, "limit", 40, "max results")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		query := strings.Join(args, " ")
		uc := usecase.NewSearchUseCase(app.docRepo, app.embed)
		results, err := uc.Execute(context.Background(), usecase.SearchInput{
			Query: query,
			Tags:  tagsFlag,
			Notebook: notebookFlag,
			Limit: limitFlag,
		})
		if err != nil {
			return err
		}
		if jsonFlag {
			type jsonResult struct {
				ID    string   `json:"id"`
				Title string   `json:"title"`
				Score float32  `json:"score"`
				Tags  []string `json:"tags"`
			}
			var jr []jsonResult
			for _, r := range results {
				jr = append(jr, jsonResult{
					ID: r.Document.ID, Title: r.Document.Title,
					Score: r.Score, Tags: r.Document.Tags,
				})
			}
			data, _ := json.MarshalIndent(jr, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		if len(results) == 0 {
			fmt.Println("No results found")
			return nil
		}
		printSearchTable(results)
		return nil
	}
	return cmd
}

func linkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <source_id> <target_id>",
		Short: "Link two notes together",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			uc := usecase.NewLinkUseCase(app.docRepo, app.linkRepo)
			if err := uc.Execute(context.Background(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Linked: %s <-> %s\n", args[0], args[1])
			return nil
		},
	}
}

func tagsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tags", Short: "List all tags"}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewTagsUseCase(app.docRepo)
		tags, err := uc.Execute(context.Background())
		if err != nil {
			return err
		}
		if jsonFlag {
			data, _ := json.MarshalIndent(tags, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		for t, c := range tags {
			fmt.Printf("  %-20s %d\n", t, c)
		}
		return nil
	}
	return cmd
}

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create a backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			uc := usecase.NewBackupUseCase(app.docRepo, defaultStoragePath)
			path, err := uc.Execute(context.Background())
			if err != nil {
				return err
			}
			fmt.Printf("Backup created: %s\n", path)
			return nil
		},
	}
}

func restoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <file>",
		Short: "Restore from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			fmt.Print("This will overwrite existing data. Continue? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled")
				return nil
			}
			uc := usecase.NewRestoreUseCase(app.docRepo)
			if err := uc.Execute(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Println("Restored successfully")
			return nil
		},
	}
}

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove all notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			fmt.Print("DELETE ALL notes? Type 'yes' to confirm: ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println("Cancelled")
				return nil
			}
			uc := usecase.NewCleanUseCase(app.docRepo)
			if err := uc.Execute(context.Background()); err != nil {
				return err
			}
			fmt.Println("All notes deleted")
			return nil
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			fmt.Printf("Editor:     %s\n", app.config.Editor)
			fmt.Printf("Ollama URL: %s\n", app.config.OllamaURL)
			fmt.Printf("Model:      %s\n", app.config.OllamaModel)
			fmt.Printf("Storage:    %s\n", defaultStoragePath)
			return nil
		},
	}
}

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run initial setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(context.Background(), defaultStoragePath)
		},
	}
}

func serveCmd() *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start web interface",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			srv := server.New(app.docRepo, app.linkRepo, app.embed, server.StaticFS)
			return srv.Start(":" + port)
		},
	}
	cmd.Flags().StringVar(&port, "port", "3003", "server port")
	return cmd
}

func handleAction(action, docID string, app *App) error {
	ctx := context.Background()
	switch action {
	case "edit":
		uc := usecase.NewEditUseCase(app.docRepo, app.embed, app.config.Editor)
		return uc.Execute(ctx, docID)
	case "delete":
		uc := usecase.NewDeleteUseCase(app.docRepo)
		return uc.Execute(ctx, docID)
	}
	return nil
}

func printDocTable(docs []*domain.Document) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tTAGS\tNOTEBOOK\tDATA")
	fmt.Fprintln(w, "--\t-----\t----\t--------\t----")
	for _, d := range docs {
		tags := strings.Join(d.Tags, ", ")
		title := d.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			d.ID, title, tags, d.Notebook, d.CreatedAt.Format("2006-01-02"))
	}
	w.Flush()
}

func printSearchTable(results []*usecase.SearchResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSCORE\tTITLE\tTAGS\tDATA")
	fmt.Fprintln(w, "--\t-----\t-----\t----\t----")
	for _, r := range results {
		d := r.Document
		tags := strings.Join(d.Tags, ", ")
		title := d.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%d%%\t%s\t%s\t%s\n",
			d.ID, int(r.Score*100), title, tags, d.CreatedAt.Format("2006-01-02"))
	}
	w.Flush()
}

func runSetup(ctx context.Context, storagePath string) error {
	os.MkdirAll(storagePath, 0755)
	os.MkdirAll(filepath.Join(storagePath, "backups"), 0755)

	cfgRepo := sqlite.NewConfigRepository(storagePath)
	if cfgRepo.Exists(ctx) {
		return nil
	}

	fmt.Println("Nota - First time setup")
	fmt.Println()

	editorName := editor.Detect()
	if !editor.IsInstalled("micro") {
		fmt.Print("Install micro editor? [Y/n] ")
		var ans string
		fmt.Scanln(&ans)
		if strings.ToLower(ans) != "n" {
			if err := editor.InstallMicro(); err != nil {
				fmt.Printf("Could not install micro: %v\n", err)
			} else {
				editorName = "micro"
			}
		}
	} else {
		editorName = "micro"
	}

	ollamaURL := "http://localhost:11434"
	ollamaModel := "nomic-embed-text:latest"

	fmt.Print("Ollama URL [http://localhost:11434]: ")
	var urlInput string
	fmt.Scanln(&urlInput)
	if urlInput != "" {
		ollamaURL = urlInput
	}

	embedSvc := ollama.New(ollamaURL, ollamaModel)
	if err := embedSvc.CheckAvailability(ctx); err != nil {
		fmt.Printf("Warning: %v\n", err)
		fmt.Print("Pull model now? [Y/n] ")
		var pullAns string
		fmt.Scanln(&pullAns)
		if strings.ToLower(pullAns) != "n" {
			fmt.Printf("Pulling %s...\n", ollamaModel)
			if err := ollama.PullModel(ctx, ollamaURL, ollamaModel); err != nil {
				fmt.Printf("Error pulling model: %v\n", err)
			} else {
				fmt.Println("Model pulled successfully")
			}
		}
	}

	cfg := &domain.Config{
		Editor:      editorName,
		OllamaURL:   ollamaURL,
		OllamaModel: ollamaModel,
		StoragePath: storagePath,
	}
	if err := cfgRepo.Save(ctx, cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Setup complete! Config saved to %s\n", filepath.Join(storagePath, "config.json"))
	return nil
}
