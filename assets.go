package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//checks if file exists
func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

//gets file name within assets
func getAssetPath(randPath, mediatype string) string {
	ext := mediaTypeToExt(mediatype)
	assetPath := randPath+ext
	return assetPath
}

//get file path from root
func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

//add path to url
func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

//extracts mediatype from mime type
func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}

	return "."+parts[1]
}