package java

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
)

// Represent a POM file
type MavenProject struct {
	Parent     MavenParentInfo `xml:"parent"`
	GroupId    string          `xml:"groupId"`
	ArtifactId string          `xml:"artifactId"`
	Version    string          `xml:"version"`
	Packaging  string          `xml:"packaging"`
}

type MavenParentInfo struct {
	GroupId string `xml:"groupId"`
	Version string `xml:"version"`
}

func (p MavenProject) GetGroupID() string {
	if p.GroupId != "" {
		return p.GroupId
	}
	return p.Parent.GroupId
}

func (p MavenProject) GetArtifactID() string {
	return p.ArtifactId
}

func (p MavenProject) GetVersion() string {
	if p.Version != "" {
		return p.Version
	}
	return p.Parent.Version
}

func (p MavenProject) GetPackaging() string {
	return p.Packaging
}

func (p MavenProject) GetID() string {
	res := p.GetGroupID() + ":" + p.GetArtifactID() + ":" + p.GetVersion()
	if p.GetPackaging() != "" && p.GetPackaging() != "jar" {
		res += p.GetPackaging()
	}
	return res
}

func parsePomFile(file string) (MavenProject, error) {
	fdata, err := ioutil.ReadFile(file)
	if err != nil {
		return MavenProject{}, err
	}
	return parsePomData(string(fdata))
}

func parsePomData(data string) (MavenProject, error) {
	project := MavenProject{}
	if err := xml.Unmarshal([]byte(data), &project); err != nil {
		return MavenProject{}, fmt.Errorf("cannot parse pom file: %s", err)
	}
	return project, nil
}
