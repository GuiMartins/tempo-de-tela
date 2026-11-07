//go:build !linux

package collectors

import (
	"context"
	"errors"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/settings"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

func (c *Collectors) run(context.Context, *store.Store, func() settings.Settings, string) error {
	return errors.New("coletores LAN disponíveis em Linux")
}
