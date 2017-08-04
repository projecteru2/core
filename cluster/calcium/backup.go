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
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

// Backup uses docker cp to copy specified directory into configured BackupDir
func (c *calcium) Backup(id, srcPath string) (*types.BackupMessage, error) {
	if c.config.BackupDir == "" {
		log.Infof("This core has no BackupDir set in config, skip backup for container %s", id)
		return nil, errors.New("BackupDir not set")
	}
	log.Debugf("Backup %s for container %s", srcPath, id)
	container, err := c.GetContainer(id)
	node, err := c.GetNode(container.Podname, container.Nodename)
	ctx := utils.ToDockerContext(node.Engine)

	resp, stat, err := node.Engine.CopyFromContainer(ctx, container.ID, srcPath)
	defer resp.Close()
	log.Debugf("Docker cp stat: %v", stat)
	if err != nil {
		log.Errorf("Error during CopyFromContainer: %v", err)
		return nil, err
	}

	appname, entrypoint, ident, err := utils.ParseContainerName(container.Name)
	if err != nil {
		log.Errorf("Error during ParseContainerName: %v", err)
		return nil, err
	}
	now := time.Now().Format("2006.01.02.15.04.05")
	baseDir := filepath.Join(c.config.BackupDir, appname, entrypoint)
	err = os.MkdirAll(baseDir, os.FileMode(0400))
	if err != nil {
		log.Errorf("Error during mkdir %s, %v", baseDir, err)
		return nil, err
	}

	filename := fmt.Sprintf("%s-%s-%s-%s.tar.gz", stat.Name, container.ShortID(), ident, now)
	backupFile := filepath.Join(baseDir, filename)
	log.Debugf("Creating %s", backupFile)
	file, err := os.Create(backupFile)
	if err != nil {
		log.Errorf("Error during create backup file %s: %v", backupFile, err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	_, err = io.Copy(gw, resp)
	if err != nil {
		log.Errorf("Error during copy resp: %v", err)
		return nil, err
	}

	return &types.BackupMessage{
		Status: "ok",
		Size:   stat.Size,
	}, nil
}
