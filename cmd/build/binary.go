package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

// compiles code in /cmd/<cmd> for the given OS and ARCH.
// Binaries will be put in /bin/<cmd>_<os>_<arch>.
func compile(ctx context.Context, cmd string, goos, goarch string) error {
	err := mgr.SerialDeps(ctx,
		run.Fn3(compile, cmd, goos, goarch),
		run.Meth(&Generate{}, (&Generate{}).All),
	)
	if err != nil {
		return err
	}

	env := sh.WithEnvironment{
		"CGO_ENABLED": "0",
		"GOOS":        goos,
		"GOARCH":      goarch,
	}

	if cgo, cgoOk := os.LookupEnv("CGO_ENABLED"); cgoOk {
		env["CGO_ENABLED"] = cgo
	}
	if goos == "" || goarch == "" {
		return fmt.Errorf("invalid os or arch") //nolint:goerr113
	}

	dst := filepath.Join("bin", fmt.Sprintf("%s_%s_%s", cmd, goos, goarch))

	ldflags := []string{
		"-w", "-s", "--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'package-operator.run/internal/version.version=%s'", appVersion),
	}

	err = shr.New(env).Run(
		"go", "build", "--ldflags", strings.Join(ldflags, " "), "--trimpath", "--mod=readonly", "-o", dst, "./cmd/"+cmd,
	)
	if err != nil {
		panic(fmt.Errorf("compiling cmd/%s: %w", cmd, err))
	}

	return nil
}

// compiles all binaries of the project for all relevant architectures.
func compileAll(ctx context.Context) error {
	return mgr.SerialDeps(ctx,
		run.Fn(compileAll),
		run.Fn3(compile, "package-operator-manager", "linux", "amd64"),
		run.Fn3(compile, "remote-phase-manager", "linux", "amd64"),
		run.Fn3(compile, "package-operator-webhook", "linux", "amd64"),
		run.Fn3(compile, "kubectl-package", "linux", "amd64"),
		run.Fn3(compile, "kubectl-package", "darwin", "amd64"),
		run.Fn3(compile, "kubectl-package", "darwin", "arm64"),
	)
}
