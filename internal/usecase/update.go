package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	githubRepo      = "gustavoSoriano/nota"
	githubAPI       = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	githubLatestURL = "https://github.com/" + githubRepo + "/releases/latest"
	downloadBase    = "https://github.com/" + githubRepo + "/releases/latest/download"
)

type UpdateResult struct {
	CurrentVersion string
	LatestVersion  string
	AlreadyLatest  bool
	Updated        bool
	IsDevBuild     bool
}

type UpdateUseCase struct {
	currentVersion string
}

func NewUpdateUseCase(currentVersion string) *UpdateUseCase {
	return &UpdateUseCase{currentVersion: currentVersion}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

func (uc *UpdateUseCase) FetchLatestVersion(ctx context.Context) (string, error) {
	// Estratégia 1: redirect do GitHub (sem quota de API)
	// GET /releases/latest redireciona para /releases/tag/vX.Y.Z
	// Extraímos a versão do header Location sem consumir rate limit
	if v, err := uc.fetchVersionViaRedirect(ctx); err == nil {
		return v, nil
	}

	// Estratégia 2: API JSON (fallback, consome quota)
	return uc.fetchVersionViaAPI(ctx)
}

func (uc *UpdateUseCase) fetchVersionViaRedirect(ctx context.Context) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // não seguir redirect, pegar o Location
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nota-cli/"+uc.currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	// Espera redirect 302 para .../releases/tag/vX.Y.Z
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no Location header")
	}

	// Extrair tag do final da URL: .../releases/tag/v0.1.1
	parts := strings.Split(strings.TrimRight(loc, "/"), "/")
	tag := parts[len(parts)-1]
	version := strings.TrimPrefix(tag, "v")
	if version == "" || version == tag {
		return "", fmt.Errorf("could not parse version from %q", loc)
	}
	return version, nil
}

func (uc *UpdateUseCase) fetchVersionViaAPI(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nota-cli/"+uc.currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return "", fmt.Errorf("could not check for updates (GitHub rate limit). Try again later or visit github.com/%s/releases", githubRepo)
	}
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("no releases found at github.com/%s", githubRepo)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsing GitHub response: %w", err)
	}

	version := strings.TrimPrefix(release.TagName, "v")
	if version == "" {
		return "", fmt.Errorf("could not determine version from tag: %q", release.TagName)
	}
	return version, nil
}

func (uc *UpdateUseCase) Execute(ctx context.Context, checkOnly bool) (*UpdateResult, error) {
	result := &UpdateResult{
		CurrentVersion: uc.currentVersion,
		IsDevBuild:     uc.currentVersion == "dev",
	}

	latestVersion, err := uc.FetchLatestVersion(ctx)
	if err != nil {
		return nil, err
	}
	result.LatestVersion = latestVersion

	currentNorm := strings.TrimPrefix(uc.currentVersion, "v")
	latestNorm := strings.TrimPrefix(latestVersion, "v")

	if currentNorm == latestNorm && !result.IsDevBuild {
		result.AlreadyLatest = true
		return result, nil
	}

	if checkOnly {
		return result, nil
	}

	// Determinar binário a baixar
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch == "aarch64" {
		goarch = "arm64"
	}

	binaryName := fmt.Sprintf("nota-%s-%s", goos, goarch)
	if goos == "windows" {
		binaryName += ".exe"
	}
	url := fmt.Sprintf("%s/%s", downloadBase, binaryName)

	// Caminho do executável atual
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not determine current executable path: %w", err)
	}

	// Baixar para arquivo temporário no mesmo diretório (para rename atômico)
	tmpFile, err := os.CreateTemp("", "nota-update-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath) // limpa se algo der errado
	}()

	if err := downloadBinary(ctx, url, tmpFile); err != nil {
		return nil, fmt.Errorf("downloading update: %w", err)
	}
	tmpFile.Close()

	// chmod +x
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return nil, fmt.Errorf("setting permissions: %w", err)
	}

	// Substituir binário atual atomicamente
	if err := replaceBinary(execPath, tmpPath); err != nil {
		return nil, err
	}

	result.Updated = true
	return result, nil
}

func downloadBinary(ctx context.Context, url string, dst *os.File) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("binary not found at %s\nMake sure a release exists for your platform (os=%s arch=%s)",
			url, runtime.GOOS, runtime.GOARCH)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return fmt.Errorf("writing download: %w", werr)
			}
			downloaded += int64(n)
			if total > 0 {
				pct := int(downloaded * 100 / total)
				fmt.Printf("\r  Downloading... %d%%", pct)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading download: %w", err)
		}
	}
	if total > 0 {
		fmt.Println() // nova linha após progresso
	}
	return nil
}

func replaceBinary(execPath, newPath string) error {
	// Tentar rename direto (funciona se mesmo filesystem)
	if err := os.Rename(newPath, execPath); err != nil {
		// Se falhar (cross-device ou permissão), tentar via cópia
		if err2 := copyFile(newPath, execPath); err2 != nil {
			return fmt.Errorf(
				"could not replace binary at %s: %w\nTry running with sudo or install manually:\n  sudo nota update",
				execPath, err,
			)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
