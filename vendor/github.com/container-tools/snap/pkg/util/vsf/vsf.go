package vsf

import (
	"io/ioutil"
	"net/http"

	_ "github.com/container-tools/snap/statik"
	"github.com/rakyll/statik/fs"
)

var vfs http.FileSystem

func init() {
	var err error
	vfs, err = fs.New()
	if err != nil {
		panic(err)
	}
}

func LoadAsString(fileName string) (string, error) {
	b, err := Load(fileName)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Load(fileName string) ([]byte, error) {
	file, err := vfs.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ioutil.ReadAll(file)
}
