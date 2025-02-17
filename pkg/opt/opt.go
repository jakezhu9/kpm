// Copyright 2023 The KCL Authors. All rights reserved.

package opt

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"kcl-lang.io/kcl-go/pkg/kcl"
	"kcl-lang.io/kpm/pkg/errors"
	"kcl-lang.io/kpm/pkg/reporter"
	"kcl-lang.io/kpm/pkg/settings"
)

// CompileOptions is the input options of 'kpm run'.
type CompileOptions struct {
	isVendor        bool
	hasSettingsYaml bool
	entries         []string
	*kcl.Option
}

// DefaultCompileOptions returns a default CompileOptions.
func DefaultCompileOptions() *CompileOptions {
	return &CompileOptions{
		Option: kcl.NewOption(),
	}
}

// AddEntry will add a compile entry file to the compiler.
func (opts *CompileOptions) AddEntry(entry string) {
	opts.entries = append(opts.entries, entry)
}

// Entrirs will return the entries of the compiler.
func (opts *CompileOptions) Entries() []string {
	return opts.entries
}

// ExtendEntries will extend the entries of the compiler.
func (opts *CompileOptions) ExtendEntries(entries []string) {
	opts.entries = append(opts.entries, entries...)
}

// SetHasSettingsYaml will set the 'hasSettingsYaml' flag.
func (opts *CompileOptions) SetHasSettingsYaml(hasSettingsYaml bool) {
	opts.hasSettingsYaml = hasSettingsYaml
}

// HasSettingsYaml will return the 'hasSettingsYaml' flag.
func (opts *CompileOptions) HasSettingsYaml() bool {
	return opts.hasSettingsYaml
}

// SetVendor will set the 'isVendor' flag.
func (opts *CompileOptions) SetVendor(isVendor bool) {
	opts.isVendor = isVendor
}

// IsVendor will return the 'isVendor' flag.
func (opts *CompileOptions) IsVendor() bool {
	return opts.isVendor
}

// PkgPath will return the home path for a kcl package during compilation
func (opts *CompileOptions) PkgPath() string {
	return opts.WorkDir
}

// SetPkgPath will set the home path for a kcl package during compilation
func (opts *CompileOptions) SetPkgPath(pkgPath string) {
	opts.Merge(kcl.WithWorkDir(pkgPath))
}

// Input options of 'kpm init'.
type InitOptions struct {
	Name     string
	InitPath string
}

func (opts *InitOptions) Validate() error {
	if len(opts.Name) == 0 {
		return errors.InvalidInitOptions
	} else if len(opts.InitPath) == 0 {
		return errors.InternalBug
	}
	return nil
}

type AddOptions struct {
	LocalPath    string
	RegistryOpts RegistryOptions
}

func (opts *AddOptions) Validate() error {
	if len(opts.LocalPath) == 0 {
		return errors.InternalBug
	} else if opts.RegistryOpts.Git != nil {
		return opts.RegistryOpts.Git.Validate()
	} else if opts.RegistryOpts.Oci != nil {
		return opts.RegistryOpts.Oci.Validate()
	} else if opts.RegistryOpts.Local != nil {
		return opts.RegistryOpts.Local.Validate()
	}
	return nil
}

type RegistryOptions struct {
	Git   *GitOptions
	Oci   *OciOptions
	Local *LocalOptions
}

type GitOptions struct {
	Url    string
	Branch string
	Commit string
	Tag    string
}

func (opts *GitOptions) Validate() error {
	if len(opts.Url) == 0 {
		return reporter.NewErrorEvent(reporter.InvalidGitUrl, errors.InvalidAddOptionsInvalidGitUrl)
	}
	return nil
}

// OciOptions for download oci packages.
// kpm will download packages from oci registry by '{Reg}/{Repo}/{PkgName}:{Tag}'.
type OciOptions struct {
	Reg     string
	Repo    string
	Tag     string
	PkgName string
}

func (opts *OciOptions) Validate() error {
	if len(opts.Repo) == 0 {
		return reporter.NewErrorEvent(reporter.InvalidRepo, errors.InvalidAddOptionsInvalidOciRepo)
	}
	return nil
}

// LocalOptions for local packages.
// kpm will find packages from local path.
type LocalOptions struct {
	Path string
}

func (opts *LocalOptions) Validate() error {
	if len(opts.Path) == 0 {
		return errors.PathIsEmpty
	}
	if _, err := os.Stat(opts.Path); err != nil {
		return err
	}
	return nil
}

const OCI_SEPARATOR = ":"

// ParseOciOptionFromString will parser '<repo_name>:<repo_tag>' into an 'OciOptions' with an OCI registry.
// the default OCI registry is 'docker.io'.
// if the 'ociUrl' is only '<repo_name>', ParseOciOptionFromString will take 'latest' as the default tag.
func ParseOciOptionFromString(oci string, tag string) (*OciOptions, error) {
	ociOpt, event := ParseOciUrl(oci)
	if event != nil && (event.Type() == reporter.IsNotUrl || event.Type() == reporter.UrlSchemeNotOci) {
		ociOpt, err := ParseOciRef(oci)
		if err != nil {
			return nil, err
		}
		if len(tag) != 0 {
			reporter.Report("kpm: kpm get version from oci reference '<repo_name>:<repo_tag>'")
			reporter.Report("kpm: arg '--tag' is invalid for oci reference")
		}
		return ociOpt, nil
	}

	ociOpt.Tag = tag

	return ociOpt, nil
}

// ParseOciOptionFromOciUrl will parse oci url into an 'OciOptions'.
// If the 'tag' is empty, ParseOciOptionFromOciUrl will take 'latest' as the default tag.
func ParseOciOptionFromOciUrl(url, tag string) (*OciOptions, *reporter.KpmEvent) {
	ociOpt, err := ParseOciUrl(url)
	if err != nil {
		return nil, err
	}
	ociOpt.Tag = tag
	return ociOpt, nil
}

// ParseOciRef will parse 'repoName:repoTag' into OciOptions,
// with default registry host 'docker.io'.
func ParseOciRef(ociRef string) (*OciOptions, error) {
	oci_address := strings.Split(ociRef, OCI_SEPARATOR)
	settings := settings.GetSettings()
	if settings.ErrorEvent != nil {
		return nil, settings.ErrorEvent
	}
	if len(oci_address) == 1 {
		return &OciOptions{
			Reg:  settings.DefaultOciRegistry(),
			Repo: oci_address[0],
		}, nil
	} else if len(oci_address) == 2 {
		return &OciOptions{
			Reg:  settings.DefaultOciRegistry(),
			Repo: oci_address[0],
			Tag:  oci_address[1],
		}, nil
	} else {
		return nil, reporter.NewEvent(reporter.IsNotRef)
	}
}

// ParseOciUrl will parse 'oci://hostName/repoName:repoTag' into OciOptions without tag.
func ParseOciUrl(ociUrl string) (*OciOptions, *reporter.KpmEvent) {
	u, err := url.Parse(ociUrl)
	if err != nil {
		return nil, reporter.NewEvent(reporter.IsNotUrl)
	}

	if u.Scheme != "oci" {
		return nil, reporter.NewEvent(reporter.UrlSchemeNotOci)
	}

	return &OciOptions{
		Reg:  u.Host,
		Repo: u.Path,
	}, nil
}

// AddStoragePathSuffix will take 'Registry/Repo/Tag' as a path suffix.
// e.g. Take '/usr/test' as input,
// and oci options is
//
//	OciOptions {
//	  Reg: 'docker.io',
//	  Repo: 'test/testRepo',
//	  Tag: 'v0.0.1'
//	}
//
// You will get a path '/usr/test/docker.io/test/testRepo/v0.0.1'.
func (oci *OciOptions) AddStoragePathSuffix(pathPrefix string) string {
	return filepath.Join(filepath.Join(filepath.Join(pathPrefix, oci.Reg), oci.Repo), oci.Tag)
}
