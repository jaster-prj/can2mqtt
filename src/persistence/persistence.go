package persistence

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"sync"

	"github.com/jaster-prj/can2mqtt/common"
)

type Persistence struct {
	mu        sync.Mutex
	configDir string
}

func NewPersistence() (*Persistence, error) {

	var err error
	basePath := os.Getenv("CAN2MQTT_STORAGE")
	if basePath == "" {
		basePath, err = os.UserConfigDir()
		if err != nil {
			return nil, err
		}
	}
	configDir := path.Join(basePath, "Can2Mqtt")
	_, err = os.Stat(configDir)
	if errors.Is(err, fs.ErrNotExist) {
		err = os.Mkdir(configDir, 0700)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return &Persistence{
		configDir: configDir,
	}, nil
}

func (p *Persistence) WriteRoutes(routesData []byte) error {
	routesFile := path.Join(p.configDir, "routes.json")
	p.mu.Lock()
	defer p.mu.Unlock()
	file, err := os.OpenFile(routesFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	if _, err = writer.Write(routesData); err != nil {
		return err
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("flush file failed: %w", err)
	}
	return nil
}

func (p *Persistence) GetChecksum() (string, error) {
	routesFile := path.Join(p.configDir, "routes.json")
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := os.ReadFile(routesFile)
	if err != nil {
		slog.Error("Failed to read routes file for checksum", "error", err)
		return "", common.ErrNoRoutesPersisted
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (p *Persistence) ReadRoutes() ([]byte, error) {
	routesFile := path.Join(p.configDir, "routes.json")
	p.mu.Lock()
	defer p.mu.Unlock()
	routesData, err := os.ReadFile(routesFile)
	if err != nil {
		slog.Error("Failed to read routes file", "error", err)
		return nil, common.ErrNoRoutesPersisted
	}
	return routesData, nil
}
