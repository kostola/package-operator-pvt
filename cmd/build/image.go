package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"pkg.package-operator.run/cardboard/modules/oci"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

func buildImage(ctx context.Context, name, registry string) error {
	buildDir, err := filepath.Abs(filepath.Join(".cache", "images", name))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(buildDir); err != nil {
		return err
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}

	binaryName := name

	// Why can't we just publish an image named `kubectl-package`? :(
	if name == "cli" {
		binaryName = "kubectl-package"
	}

	self := run.Fn2(buildImage, name, registry)
	if err := mgr.SerialDeps(ctx, self,
		run.Fn3(compile, binaryName, "linux", "amd64"),
	); err != nil {
		return err
	}

	for _, file := range []struct {
		dst, src string
	}{
		{
			dst: filepath.Join(buildDir, binaryName),
			src: filepath.Join("bin", binaryName+"_linux_amd64"),
		},
		{
			dst: filepath.Join(buildDir, "passwd"),
			src: filepath.Join("config", "images", "passwd"),
		},
		{
			dst: filepath.Join(buildDir, "Containerfile"),
			src: filepath.Join("config", "images", name+".Containerfile"),
		},
	} {
		if err := shr.Copy(file.dst, file.src); err != nil {
			return err
		}
	}

	return oci.NewOCI(name, buildDir,
		oci.WithContainerFile("Containerfile"),
		oci.WithTags{appVersion},
		oci.WithRegistries{registry},
	).Build()
}

func pushImage(ctx context.Context, name, registry string) error {
	imgPath, err := filepath.Abs(filepath.Join(".cache", "images", name))
	if err != nil {
		return err
	}

	self := run.Fn2(pushImage, name, registry)
	if err := mgr.SerialDeps(ctx, self,
		run.Fn2(buildImage, name, registry),
	); err != nil {
		return err
	}

	return oci.NewOCI(name, imgPath,
		oci.WithTags{appVersion},
		oci.WithRegistries{registry},
		// push via crane, because podman does not support HTTP pushes for local dev.
		oci.WithCranePush{},
	).Push()
}

func imageURL(registry, name, version string) string {
	url := os.Getenv(strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE")
	if url == "" {
		return fmt.Sprintf("%s/%s:%s", registry, name, version)
	}
	return url
}

func version() (string, error) {
	// Use version from VERSION env if present, use "git describe" elsewise.
	if pkoVersion := strings.TrimSpace(os.Getenv("VERSION")); pkoVersion != "" {
		return pkoVersion, nil
	}

	version, err := shr.New(sh.WithLogger{}).Output("git", "describe", "--tags")
	if err != nil {
		return "", fmt.Errorf("git describe: %w", err)
	}

	// Depending on what process was used the last tag may either be a version for
	// the main module (eg `v1.6.6`) or a version for a submodule (eg `apis/v1.6.6`).
	return path.Base(strings.TrimSpace(version)), nil
}

func mustVersion() string {
	v, err := version()
	run.Must(err)
	return v
}
