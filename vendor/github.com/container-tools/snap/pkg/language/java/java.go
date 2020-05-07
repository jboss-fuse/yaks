package java

import (
	"fmt"
	"io"
	"os/exec"
	"path"

	"github.com/container-tools/snap/pkg/language"
	"github.com/container-tools/snap/pkg/util/log"
)

type JavaBindings struct {
	stdOut io.Writer
	stdErr io.Writer
}

var (
	logger = log.WithName("java-deployer")
)

func NewJavaBindings(stdOut, stdErr io.Writer) language.Bindings {
	return &JavaBindings{
		stdOut: stdOut,
		stdErr: stdErr,
	}
}

func (d *JavaBindings) Deploy(source, destination string) error {
	logger.Infof("Executing maven release phase on project %s", source)
	cmd := exec.Command("./mvnw", "deploy", "-DskipTests", fmt.Sprintf("-DaltDeploymentRepository=snapshot-repo::default::file:%s", destination))
	cmd.Dir = source
	cmd.Stdout = d.stdOut
	cmd.Stderr = d.stdErr
	err := cmd.Run()
	if err != nil {
		return err
	}
	logger.Infof("Maven release phase completed for project %s", source)
	return nil
}

func (d *JavaBindings) GetID(source string) (string, error) {
	pomLocation := path.Join(source, "pom.xml")
	project, err := parsePomFile(pomLocation)
	if err != nil {
		return "", err
	}
	return project.GetID(), nil
}
