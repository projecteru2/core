package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListAllExecutableFiles(t *testing.T) {
	// TODO
	files, err := ListAllExecutableFiles("/Users/zheyin.liang/Documents/eru/plugins")
	assert.Nil(t, err)
	for _, file := range files {
		fmt.Println(file)
	}
}
