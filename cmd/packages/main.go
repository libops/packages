package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/build"
	"github.com/libops/packages/internal/buildapk"
	"github.com/libops/packages/internal/catalog"
	"github.com/libops/packages/internal/config"
	"github.com/libops/packages/internal/githubapi"
	"github.com/libops/packages/internal/pipeline"
	"github.com/libops/packages/internal/registry"
)

func execute(ctx context.Context, cfg config.Environment, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: packages <generate|plan|metadata|build|publish|manifest|test-package|update|cleanup|review-lines|builder-plan|builder-build|builder-manifest|pin-builder|check-pins|fmt-check>")
	}
	switch args[0] {
	case "generate":
		return pipeline.Generate(".", len(args) > 1 && args[1] == "--check")
	case "check-coverage":
		if len(args) != 2 {
			return fmt.Errorf("check-coverage requires profile path")
		}
		return pipeline.CheckCoverage(args[1])
	case "write-installers":
		if len(args) != 3 {
			return fmt.Errorf("write-installers requires directory and primary package")
		}
		return buildhelpers.WriteInstallers(args[1], args[2])
	case "docs-packages":
		return build.DocumentPackages(ctx, cfg, ".")
	case "test-install":
		return build.TestInstall(ctx)
	case "fmt-check":
		return pipeline.CheckFormat(ctx)
	case "plan":
		selection, err := pipeline.Select(ctx, cfg, ".")
		if err != nil {
			return err
		}
		data, err := json.Marshal(selection.Targets)
		if err == nil {
			fmt.Println("targets=" + string(data))
			fmt.Printf("local-builder=%t\n", selection.LocalBuilder)
		}
		return err
	case "metadata":
		if len(args) != 2 {
			return fmt.Errorf("metadata requires target")
		}
		c, err := catalog.Load(".")
		if err != nil {
			return err
		}
		p, err := c.Metadata(".", args[1])
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(p, "", "  ")
		if err == nil {
			fmt.Println(string(data))
		}
		return err
	case "build", "publish", "manifest", "login":
		return build.Task(ctx, cfg, args[0])
	case "test-package":
		return build.Task(ctx, cfg, "test")
	case "builder-plan", "builder-build", "builder-manifest":
		return build.Builder(ctx, cfg, args[0])
	case "revise-base":
		return pipeline.ReviseBase(ctx, ".", cfg.Get("TARGET"))
	case "review-lines":
		return pipeline.Review(ctx, cfg, cfg.Get("REPORT") == "1")
	case "pin-builder":
		if len(args) < 3 {
			return fmt.Errorf("pin-builder requires target and digest")
		}
		return githubapi.PinBuilder(ctx, cfg, args[1], args[2], len(args) > 3 && args[3] == "--pull-request")
	case "cleanup":
		return registry.Cleanup(ctx, cfg)
	case "build-apk":
		return buildapk.Run(ctx, cfg)
	case "check-pins":
		return buildhelpers.CheckPins(".")
	case "update":
		if len(args) != 2 {
			return fmt.Errorf("update requires target")
		}
		return (buildhelpers.Updater{Context: ctx}).Update(".", args[1])
	}
	return fmt.Errorf("unknown command %q", args[0])
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := execute(ctx, config.Snapshot(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
