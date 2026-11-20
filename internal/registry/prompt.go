package registry

import (
	"bufio"
	"context"
	"github.com/libops/packages/internal/auth"
	"github.com/libops/packages/internal/config"
	"os"
)

func promptToken(ctx context.Context, cfg config.Environment) (string, error) {
	return auth.Prompt(ctx, cfg, "GitHub package cleanup token", true, bufio.NewReader(os.Stdin))
}
