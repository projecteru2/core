package calcium

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Backup uses docker cp to copy specified directory into configured BackupDir
func (c *calcium) Backup(id, srcPath string) (*types.BackupMessage, error) {
	if c.config.BackupDir == "" {
		log.Infof("[Backup] This core has no BackupDir set in config, skip backup for container %s", id)
		return nil, errors.New("BackupDir not set")
	}
	log.Debugf("[Backup] Backup %s for container %s", srcPath, id)
	container, err := c.GetContainer(id)
	if err != nil {
		return nil, err
	}
	node, err := c.GetNode(container.Podname, container.Nodename)
	if err != nil {
		return nil, err
	}
	ctx := utils.ToDockerContext(node.Engine)

	resp, stat, err := node.Engine.CopyFromContainer(ctx, container.ID, srcPath)
	defer resp.Close()
	log.Debugf("[Backup] Docker cp stat: %v", stat)
	if err != nil {
		log.Errorf("[Backup] Error during CopyFromContainer: %v", err)
		return nil, err
	}

	appname, entrypoint, ident, err := utils.ParseContainerName(container.Name)
	if err != nil {
		log.Errorf("[Backup] Error during ParseContainerName: %v", err)
		return nil, err
	}
	now := time.Now().Format("2006.01.02.15.04.05")
	baseDir := filepath.Join(c.config.BackupDir, appname, entrypoint)
	err = os.MkdirAll(baseDir, os.FileMode(0700)) // drwx------
	if err != nil {
		log.Errorf("[Backup] Error during mkdir %s, %v", baseDir, err)
		return nil, err
	}

	filename := fmt.Sprintf("%s-%s-%s-%s.tar.gz", stat.Name, container.ShortID(), ident, now)
	backupFile := filepath.Join(baseDir, filename)
	log.Debugf("[Backup] Creating %s", backupFile)
	file, err := os.Create(backupFile)
	if err != nil {
		log.Errorf("[Backup] Error during create backup file %s: %v", backupFile, err)
		return nil, err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	_, err = io.Copy(gw, resp)
	if err != nil {
		log.Errorf("[Backup] Error during copy resp: %v", err)
		return nil, err
	}

	return &types.BackupMessage{
		Status: "ok",
		Size:   stat.Size,
		Path:   backupFile,
	}, nil
}
