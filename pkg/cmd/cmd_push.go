// Copyright 2023 The KCL Authors. All rights reserved.

package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/urfave/cli/v2"
	"kcl-lang.io/kpm/pkg/errors"
	"kcl-lang.io/kpm/pkg/oci"
	"kcl-lang.io/kpm/pkg/opt"
	pkg "kcl-lang.io/kpm/pkg/package"
	"kcl-lang.io/kpm/pkg/reporter"
	"kcl-lang.io/kpm/pkg/settings"
	"kcl-lang.io/kpm/pkg/utils"
)

// NewPushCmd new a Command for `kpm push`.
func NewPushCmd(settings *settings.Settings) *cli.Command {
	return &cli.Command{
		Hidden: false,
		Name:   "push",
		Usage:  "push kcl package to OCI registry.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  FLAG_TAR_PATH,
				Usage: "a kcl file as the compile entry file",
			},
			// '--vendor' will trigger the vendor mode
			// In the vendor mode, the package search path is the subdirectory 'vendor' in current package.
			// In the non-vendor mode, the package search path is the $KCL_PKG_PATH.
			&cli.BoolFlag{
				Name:  FLAG_VENDOR,
				Usage: "push in vendor mode",
			},
		},
		Action: func(c *cli.Context) error {
			return KpmPush(c, settings)
		},
	}
}

func KpmPush(c *cli.Context, settings *settings.Settings) error {
	localTarPath := c.String(FLAG_TAR_PATH)
	ociUrl := c.Args().First()

	var err error

	if len(localTarPath) == 0 {
		// If the tar package to be pushed is not specified,
		// the current kcl package is packaged into tar and pushed.
		err = pushCurrentPackage(ociUrl, c.Bool(FLAG_VENDOR), settings)
	} else {
		// Else push the tar package specified.
		err = pushTarPackage(ociUrl, localTarPath, c.Bool(FLAG_VENDOR), settings)
	}

	if err != nil {
		return err
	}

	return nil
}

// genDefaultOciUrlForKclPkg will generate the default oci url from the current package.
func genDefaultOciUrlForKclPkg(pkg *pkg.KclPkg) (string, error) {
	settings := settings.GetSettings()
	if settings.ErrorEvent != nil {
		return "", settings.ErrorEvent
	}

	urlPath := utils.JoinPath(settings.DefaultOciRepo(), pkg.GetPkgName())

	u := &url.URL{
		Scheme: oci.OCI_SCHEME,
		Host:   settings.DefaultOciRegistry(),
		Path:   urlPath,
	}

	return u.String(), nil
}

// pushCurrentPackage will push the current package to the oci registry.
func pushCurrentPackage(ociUrl string, vendorMode bool, settings *settings.Settings) error {
	pwd, err := os.Getwd()

	if err != nil {
		reporter.ReportEventToStderr(reporter.NewEvent(reporter.Bug, "internal bug: failed to load working directory"))
		return err
	}
	// 1. Load the current kcl packege.
	kclPkg, err := pkg.LoadKclPkg(pwd)

	if err != nil {
		reporter.ReportEventToStderr(reporter.NewEvent(reporter.FailedLoadKclMod, fmt.Sprintf("failed to load package in '%s'", pwd)))
		return err
	}

	// 2. push the package
	return pushPackage(ociUrl, kclPkg, vendorMode, settings)
}

// pushTarPackage will push the kcl package in tarPath to the oci registry.
// If the tar in 'tarPath' is not a kcl package tar, pushTarPackage will return an error.
func pushTarPackage(ociUrl, localTarPath string, vendorMode bool, settings *settings.Settings) error {
	var kclPkg *pkg.KclPkg
	var err error

	// clean the temp dir used to untar kcl package tar file.
	defer func() {
		if kclPkg != nil && utils.DirExists(kclPkg.HomePath) {
			err = os.RemoveAll(kclPkg.HomePath)
			if err != nil {
				err = errors.InternalBug
			}
		}
	}()

	// 1. load the kcl package from the tar path.
	kclPkg, err = pkg.LoadKclPkgFromTar(localTarPath)
	if err != nil {
		return err
	}

	// 2. push the package
	return pushPackage(ociUrl, kclPkg, vendorMode, settings)
}

// pushPackage will push the kcl package to the oci registry.
// 1. pushPackage will package the current kcl package into default tar path.
// 2. If the oci url is not specified, generate the default oci url from the current package.
// 3. Generate the OCI options from oci url and the version of current kcl package.
// 4. Push the package to the oci registry.
func pushPackage(ociUrl string, kclPkg *pkg.KclPkg, vendorMode bool, settings *settings.Settings) error {
	// 1. Package the current kcl package into default tar path.
	tarPath, err := kclPkg.PackageCurrentPkgPath(vendorMode)
	if err != nil {
		return err
	}

	// clean the tar path.
	defer func() {
		if kclPkg != nil && utils.DirExists(tarPath) {
			err = os.RemoveAll(tarPath)
			if err != nil {
				err = errors.InternalBug
			}
		}
	}()

	// 2. If the oci url is not specified, generate the default oci url from the current package.
	if len(ociUrl) == 0 {
		ociUrl, err = genDefaultOciUrlForKclPkg(kclPkg)
		if err != nil || len(ociUrl) == 0 {
			reporter.Report("kpm: failed to generate default oci url for current package.")
			reporter.Report("kpm: run 'kpm push help' for more information.")
			return errors.FailedPushToOci
		}
	}

	// 3. Generate the OCI options from oci url and the version of current kcl package.
	ociOpts, err := opt.ParseOciOptionFromOciUrl(ociUrl, kclPkg.GetPkgTag())
	if err != (*reporter.KpmEvent)(nil) {
		return reporter.NewErrorEvent(
			reporter.UnsupportOciUrlScheme,
			errors.InvalidOciUrl,
			"only support url scheme 'oci://'.",
		)
	}

	reporter.Report("kpm: package '" + kclPkg.GetPkgName() + "' will be pushed.")
	// 4. Push it.
	err = oci.Push(tarPath, ociOpts.Reg, ociOpts.Repo, ociOpts.Tag, settings)
	if err != (*reporter.KpmEvent)(nil) {
		return err
	}

	return nil
}
