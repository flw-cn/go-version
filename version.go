// version module provides some API to help Go Application get it's version
// with zero-cost.
//
// After Go 1.15, version information for applications written in Go is
// available from its own binary. This is a very useful feature, but I've
// noticed that almost no one in the community at large uses it.
//
// I think this is probably because runtime/debug.BuildInfo is harder to use
// directly, and the community doesn't have a ready-made library to simplify
// this. So I wrote this module to help people easily get the version information
// of their Go programs. Of course, based on the go module version control policy
// and semantic version.
//
// See also:
//  * Go Modules Reference: https://go.dev/ref/mod
//  * Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html
//
package version

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// VersionType indicate the type of Version, there are six valid VersionTypes:
//   * Devel
//   * Release
//   * PreRelease
//   * PseudoBaseNoTag
//   * PseudoBaseRelease
//   * PseudoBasePreRelease
//
type VersionType int

const (
	Devel                VersionType = iota // built via `go build`
	Release                                 // built via `go install` with a release tag
	PreRelease                              // built via `go install` with a pre-release tag
	PseudoBaseNoTag                         // built via `go install` without any tag
	PseudoBaseRelease                       // built on several patches after a release tag
	PseudoBasePreRelease                    // built on several patches after a pre-release tag
	ErrorVersion                            // some errors have occurred
)

// ModVersion represents the information retrieved from debug.Module.Version.
type ModVersion struct {
	Type     VersionType
	Tag      string
	CommitID string
	Time     time.Time
}

// VcsInfo represents the information retrieved from debug.BuildSetting.
type VcsInfo struct {
	VCS        string
	Revision   string
	IsDirty    bool
	LastCommit time.Time
}

// Brief provides the field to render a brief version line.
type Brief struct {
	AppName    string
	ModulePath string
	AppVersion string
	GoVersion  string
}

// Detail provides the field to render a detail version information.
type Detail struct {
	Brief
	ModVersion
	VcsInfo
	TagRemarks string
}

// GetAppVersion get Go Application Version from Go binary via debug.BuildInfo.
//
// A Go Module Version string layout is one of follow formats:
//   * dirty vcs work directory: (devel)
//   * release version: vX.Y.Z
//   * pre-release version: v1.2.3-RC1
//   * pseudo version:
//      - untagged branch: v0.0.0-YYYYmmddHHMMSS-aabbccddeeff
//      - base on release version: vX.Y.(Z+1)-0.YYYYmmddHHMMSS-aabbccddeeff
//      - base on pre-release version: vX.Y.Z-RC1.0.YYYYmmddHHMMSS-aabbccddeeff
//
// See also: https://go.dev/ref/mod#glossary
//
func GetAppVersion(version string) (verInfo *ModVersion) {
	verInfo = &ModVersion{}

	if version == "" {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return nil
		}
		version = info.Main.Version
	}

	parts := strings.Split(version, "-")
	tag := parts[0]
	n := len(parts)
	if n < 3 { // this is not a pseudo version
		if tag == "(devel)" {
			verInfo.Type = Devel
		} else if strings.Contains(tag, "-") {
			verInfo.Type = PreRelease
		} else {
			verInfo.Type = Release
		}
		return
	}

	verInfo.CommitID = parts[n-1]
	timeStr := parts[n-2]
	actualLen := len(timeStr)
	expectLen := len("YYYYmmddHHMMSS")
	if actualLen < expectLen {
		return nil
	}

	t, err := time.Parse("20060102150405", timeStr[actualLen-expectLen:actualLen])
	if err != nil {
		return nil
	}

	verInfo.Time = t

	if actualLen == expectLen {
		verInfo.Type = PseudoBaseNoTag
		return
	}

	if actualLen == expectLen+2 {
		parts := strings.Split(tag, ".")
		patch, _ := strconv.Atoi(parts[2])
		if patch > 0 {
			patch = patch - 1
		}
		verInfo.Tag = parts[0] + "." + parts[1] + "." + strconv.Itoa(patch)
		verInfo.Type = PseudoBaseRelease
		return
	}

	tagLen := len(version) - len(".0.yyyymmddhhmmss-aabbccddeeff")
	verInfo.Tag = version[0:tagLen]
	verInfo.Type = PseudoBasePreRelease

	return
}

// GetVcsInfo extract VCS information from debug.BuildSetting.
// if settings is nil, GetVcsInfo will call debug.ReadBuildInfo() by itself.
//
func GetVcsInfo(settings []debug.BuildSetting) *VcsInfo {
	if settings == nil {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return nil
		}
		settings = info.Settings
	}

	vcs := "unknown"
	revision := "unknown"
	var commitTime time.Time
	dirty := false

	for _, s := range settings {
		switch s.Key {
		case "vcs":
			vcs = s.Value
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			t, e := time.Parse(time.RFC3339, s.Value)
			if e == nil {
				commitTime = t
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = true
			}
		}
	}

	return &VcsInfo{
		VCS:        vcs,
		Revision:   revision,
		LastCommit: commitTime,
		IsDirty:    dirty,
	}
}

// PrintVersion combines information from GetAppVersion() and GetVcsInfo(), it
// provides version information in a human-readable manner.
// User-supplied writer can extend the scope of PrintVersion, typically with os.Stderr.
//
// brief and detail are two templates to allow control of the output format.
// PrintVersion uses text/template to generate the output and provides inputs
// for brief and detail respectively.
//
// The default brief template is:
//    {{.AppName}} version {{.AppVersion}}, built with {{.GoVersion}}
//
// Tnd default detail template is:
//    WARNING! This is not a release version, it's built from a {{.TagRemarks}}.
//
//    VCS information:
//    VCS:         {{.VCS}}
//    Module path: {{.ModulePath}}
//    Commit time: {{.LastCommit.Local.Format "2006-01-02 15:04:05 MST"}}
//    Revision id: {{.Revision}}
//
//    Please visit {{.ModulePath}} to get updates.
//
// PrintVersion always evaluates brief, and only evaluates detail if the tag is
// not a release and pre-release tag.
//
func PrintVersion(w io.Writer, brief, detail string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Fprintln(w, "Can't get build info.")
		return
	}

	appName := filepath.Base(info.Path)

	if brief == "" {
		brief = "{{.AppName}} version {{.AppVersion}}, built with {{.GoVersion}}\n"
	}

	tmpl, err := template.New("brief").Parse(brief)
	if err != nil {
		panic(fmt.Sprintf("brief template error: %v", err))
	}

	briefInfo := Brief{
		AppName:    appName,
		ModulePath: info.Path,
		AppVersion: info.Main.Version,
		GoVersion:  info.GoVersion,
	}

	err = tmpl.Execute(w, briefInfo)

	if err != nil {
		panic(fmt.Sprintf("brief template error: %v", err))
	}

	vcsInfo := GetVcsInfo(info.Settings)
	verInfo := GetAppVersion(info.Main.Version)
	if verInfo == nil || vcsInfo == nil {
		return
	}

	tagRemarks := ""

	switch verInfo.Type {
	case Release, PreRelease:
		// info.Settings can't contains any valid VCS information. just return
		return
	case ErrorVersion:
		tagRemarks = "unknown branch"
	case Devel:
		if vcsInfo.IsDirty {
			tagRemarks = "dirty working copy"
		} else {
			tagRemarks = "clean working copy"
		}
	case PseudoBaseNoTag, PseudoBaseRelease, PseudoBasePreRelease:
		if verInfo.Type == PseudoBaseNoTag {
			tagRemarks = "untagged branch"
		} else {
			tagRemarks = "branch base on tag " + verInfo.Tag
		}
		vcsInfo.Revision = verInfo.CommitID
		vcsInfo.LastCommit = verInfo.Time
	}

	if detail == "" {
		detail = `WARNING! This is not a release version, it's built from a {{.TagRemarks}}.

VCS information:
VCS:         {{.VCS}}
Module path: {{.ModulePath}}
Commit time: {{.LastCommit.Local.Format "2006-01-02 15:04:05 MST"}}
Revision id: {{.Revision}}

Please visit {{.ModulePath}} to get updates.
`
	}

	tmpl, err = template.New("detail").Parse(detail)
	if err != nil {
		panic(fmt.Sprintf("detail template error: %v", err))
	}

	err = tmpl.Execute(w, Detail{
		Brief:      briefInfo,
		ModVersion: *verInfo,
		VcsInfo:    *vcsInfo,
		TagRemarks: tagRemarks,
	})

	if err != nil {
		panic(fmt.Sprintf("detail template error: %v", err))
	}
}
