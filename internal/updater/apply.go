package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrNoAssetForPlatform = errors.New("nenhum arquivo compatível encontrado para este sistema operacional e arquitetura")
	ErrChecksumMismatch   = errors.New("falha na validação de integridade SHA256: o hash não corresponde ao release oficial")
	ErrPermissionDenied   = errors.New("permissão negada para atualizar o binário: execute 'sudo sopro update'")
)

// Apply faz o download, validação e substituição atômica do binário atual pelo da release informada.
func Apply(ctx context.Context, release *ReleaseInfo) error {
	if release == nil || release.AssetURL == "" {
		return ErrNoAssetForPlatform
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("não foi possível determinar o caminho do binário atual: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("não foi possível resolver link do binário: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "sopro-update-*")
	if err != nil {
		return fmt.Errorf("falha ao criar diretório temporário: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, release.AssetName)
	if err := downloadFile(ctx, release.AssetURL, archivePath); err != nil {
		return fmt.Errorf("falha ao baixar release: %w", err)
	}

	if release.ChecksumURL != "" {
		checksumsPath := filepath.Join(tmpDir, "checksums.txt")
		if err := downloadFile(ctx, release.ChecksumURL, checksumsPath); err == nil {
			if err := verifyChecksum(archivePath, checksumsPath, release.AssetName); err != nil {
				return err
			}
		}
	}

	binaryData, err := extractBinary(archivePath, release.AssetName)
	if err != nil {
		return fmt.Errorf("falha ao extrair binário do arquivo: %w", err)
	}

	return replaceBinary(execPath, binaryData)
}

func downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "sopro-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download falhou com status HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyChecksum(archivePath, checksumsPath, assetName string) error {
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	hasher.Write(archiveData)
	actualHash := hex.EncodeToString(hasher.Sum(nil))

	checksumsData, err := os.ReadFile(checksumsPath)
	if err != nil {
		return nil // Se não conseguiu ler checksums.txt, prossegue
	}

	lines := strings.Split(string(checksumsData), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			expectedHash := strings.ToLower(fields[0])
			if expectedHash != actualHash {
				return fmt.Errorf("%w (esperado: %s, obtido: %s)", ErrChecksumMismatch, expectedHash, actualHash)
			}
			return nil
		}
	}
	return nil
}

func extractBinary(archivePath, assetName string) ([]byte, error) {
	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		return extractTarGz(archivePath)
	}
	if strings.HasSuffix(assetName, ".zip") {
		return extractZip(archivePath)
	}
	return nil, fmt.Errorf("formato de arquivo não suportado: %s", assetName)
}

func extractTarGz(archivePath string) ([]byte, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Base(header.Name)
		if name == "sopro" || name == "sopro.exe" {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
	return nil, errors.New("binário do sopro não encontrado dentro do tarball")
}

func extractZip(archivePath string) ([]byte, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	for _, file := range zr.File {
		name := filepath.Base(file.Name)
		if name == "sopro" || name == "sopro.exe" {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
	return nil, errors.New("binário do sopro não encontrado dentro do zip")
}

func replaceBinary(execPath string, newBinaryData []byte) error {
	dir := filepath.Dir(execPath)
	tempFile, err := os.CreateTemp(dir, "sopro-new-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return ErrPermissionDenied
		}
		// Se falhou por permissão no diretório, tenta criar no temp do SO
		tempFile, err = os.CreateTemp("", "sopro-new-*")
		if err != nil {
			return fmt.Errorf("falha ao criar arquivo temporário: %w", err)
		}
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(newBinaryData); err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()

	if err := os.Chmod(tempPath, 0755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("não foi possível mover binário atual no Windows: %w", err)
		}
		if err := copyFile(tempPath, execPath); err != nil {
			_ = os.Rename(oldPath, execPath)
			return err
		}
		return nil
	}

	// Unix / Linux: tenta rename atômico primeiro
	if err := os.Rename(tempPath, execPath); err != nil {
		// Fallback: cópia direta com truncagem
		if err := copyFile(tempPath, execPath); err != nil {
			if errors.Is(err, os.ErrPermission) {
				return ErrPermissionDenied
			}
			return fmt.Errorf("falha ao substituir binário em %s: %w", execPath, err)
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

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
