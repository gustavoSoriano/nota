package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/soriano/nota/internal/domain"
	"github.com/soriano/nota/internal/infra/editor"
	"github.com/soriano/nota/internal/infra/ollama"
	"github.com/soriano/nota/internal/infra/sqlite"
	"github.com/soriano/nota/internal/tui"
	"github.com/soriano/nota/internal/usecase"
)

var (
	tagsFlag      []string
	grupoFlag     string
	catFlag       string
	contentFlag   string
	sortFlag      string
	jsonFlag      bool
	rawFlag       bool
	forceFlag     bool
)

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
		Use:   "nota",
		Short: "CLI Knowledge Base powered by markdown",
		Long:  "Nota is a CLI tool for storing and searching markdown notes with semantic search.",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringSliceVar(&tagsFlag, "tags", nil, "tags (comma-separated)")
	root.PersistentFlags().StringVar(&grupoFlag, "grupo", "", "group")
	root.PersistentFlags().StringVar(&catFlag, "cat", "", "category")

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
	cmd.Flags().StringVarP(&contentFlag, "content", "c", "", "content (skip editor)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewCreateUseCase(app.docRepo, app.embed, app.config.Editor)
		doc, err := uc.Execute(context.Background(), usecase.CreateInput{
			Tags:      tagsFlag,
			Grupo:     grupoFlag,
			Categoria: catFlag,
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
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			stat, _ := os.Stdin.Stat()
			fromPipe := stat.Mode()&os.ModeCharDevice == 0
			uc := usecase.NewSaveUseCase(app.docRepo, app.embed)
			doc, err := uc.Execute(context.Background(), usecase.SaveInput{
				Content:   strings.Join(args, " "),
				Tags:      tagsFlag,
				Grupo:     grupoFlag,
				Categoria: catFlag,
				FromPipe:  fromPipe,
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
	return &cobra.Command{
		Use:   "edit [filter]",
		Short: "Edit a note",
		Aliases: []string{"e"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			docs, err := app.docRepo.List(context.Background(), domain.ListFilter{Limit: 100})
			if err != nil {
				return err
			}
			if len(docs) == 0 {
				fmt.Println("No notes found")
				return nil
			}
			selected, err := tui.RunFuzzyPicker(docs, "Edit")
			if err != nil || selected == nil {
				return err
			}
			uc := usecase.NewEditUseCase(app.docRepo, app.embed, app.config.Editor)
			if err := uc.Execute(context.Background(), selected.ID); err != nil {
				return err
			}
			fmt.Printf("Updated: %s\n", selected.Title)
			return nil
		},
	}
}

func openCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open [id]",
		Short: "Open and view a note",
		Aliases: []string{"o"},
	}
	cmd.Flags().BoolVar(&rawFlag, "raw", false, "output raw markdown")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		ctx := context.Background()

		if len(args) > 0 {
			uc := usecase.NewOpenUseCase(app.docRepo)
			doc, err := uc.Execute(ctx, args[0])
			if err != nil {
				return err
			}
			if rawFlag {
				fmt.Println(doc.Content)
				return nil
			}
			linked, _ := app.linkRepo.GetLinked(ctx, doc.ID)
			action, err := tui.RunViewer(doc, linked)
			if err != nil {
				return err
			}
			return handleAction(action, doc.ID, app)
		}

		docs, err := app.docRepo.List(ctx, domain.ListFilter{Limit: 100})
		if err != nil {
			return err
		}
		selected, err := tui.RunFuzzyPicker(docs, "Open")
		if err != nil || selected == nil {
			return err
		}
		uc := usecase.NewOpenUseCase(app.docRepo)
		doc, err := uc.Execute(ctx, selected.ID)
		if err != nil {
			return err
		}
		if rawFlag {
			fmt.Println(doc.Content)
			return nil
		}
		linked, _ := app.linkRepo.GetLinked(ctx, doc.ID)
		action, err := tui.RunViewer(doc, linked)
		if err != nil {
			return err
		}
		return handleAction(action, doc.ID, app)
	}
	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a note",
		Aliases: []string{"d"},
	}
	cmd.Flags().BoolVar(&forceFlag, "force", false, "skip confirmation and fuzzy finder")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		ctx := context.Background()

		if forceFlag && len(args) > 0 {
			uc := usecase.NewDeleteUseCase(app.docRepo)
			return uc.Execute(ctx, args[0])
		}

		var id string
		if len(args) > 0 {
			id = args[0]
		} else {
			docs, err := app.docRepo.List(ctx, domain.ListFilter{Limit: 100})
			if err != nil {
				return err
			}
			selected, err := tui.RunFuzzyPicker(docs, "Delete")
			if err != nil || selected == nil {
				return err
			}
			id = selected.ID
		}

		fmt.Printf("Delete note %s? [y/N] ", id)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("Cancelled")
			return nil
		}

		uc := usecase.NewDeleteUseCase(app.docRepo)
		return uc.Execute(ctx, id)
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
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := ensureSetup()
		if err != nil {
			return err
		}
		uc := usecase.NewListUseCase(app.docRepo)
		docs, err := uc.Execute(context.Background(), usecase.ListInput{
			Tags:      tagsFlag,
			Grupo:     grupoFlag,
			Categoria: catFlag,
			Sort:      sortFlag,
			Limit:     20,
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
		for _, d := range docs {
			tags := strings.Join(d.Tags, ", ")
			meta := ""
			if tags != "" {
				meta += " · " + tags
			}
			if d.Grupo != "" {
				meta += " · " + d.Grupo
			}
			fmt.Printf("  %s  %s%s  %s\n",
				fmt.Sprintf("%-8s", d.ID),
				d.Title,
				meta,
				d.CreatedAt.Format("2006-01-02"),
			)
		}
		return nil
	}
	return cmd
}

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Semantic search notes",
		Aliases: []string{"s"},
		Args:  cobra.MinimumNArgs(1),
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
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
			Grupo: grupoFlag,
			Limit: 20,
		})
		if err != nil {
			return err
		}
		if jsonFlag {
			type jsonResult struct {
				ID    string  `json:"id"`
				Title string  `json:"title"`
				Score float32 `json:"score"`
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
		selected, err := tui.RunSearchTUI(results, query)
		if err != nil || selected == nil {
			return err
		}
		openUC := usecase.NewOpenUseCase(app.docRepo)
		doc, err := openUC.Execute(context.Background(), selected.Document.ID)
		if err != nil {
			return err
		}
		linked, _ := app.linkRepo.GetLinked(context.Background(), doc.ID)
		action, err := tui.RunViewer(doc, linked)
		if err != nil {
			return err
		}
		return handleAction(action, doc.ID, app)
	}
	return cmd
}

func linkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link",
		Short: "Link two notes together",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ensureSetup()
			if err != nil {
				return err
			}
			ctx := context.Background()
			docs, err := app.docRepo.List(ctx, domain.ListFilter{Limit: 100})
			if err != nil {
				return err
			}
			fmt.Println("Select source note:")
			source, err := tui.RunFuzzyPicker(docs, "Link Source")
			if err != nil || source == nil {
				return err
			}
			fmt.Println("Select target note:")
			target, err := tui.RunFuzzyPicker(docs, "Link Target")
			if err != nil || target == nil {
				return err
			}
			uc := usecase.NewLinkUseCase(app.docRepo, app.linkRepo)
			if err := uc.Execute(ctx, source.ID, target.ID); err != nil {
				return err
			}
			fmt.Printf("Linked: %s <-> %s\n", source.Title, target.Title)
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
