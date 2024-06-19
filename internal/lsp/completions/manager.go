package completions

import (
	"fmt"

	"github.com/open-policy-agent/opa/storage"

	"github.com/styrainc/regal/internal/lsp/cache"
	"github.com/styrainc/regal/internal/lsp/completions/providers"
	"github.com/styrainc/regal/internal/lsp/rego"
	"github.com/styrainc/regal/internal/lsp/types"
)

type Manager struct {
	c         *cache.Cache
	opts      *ManagerOptions
	providers []Provider
}

type ManagerOptions struct{}

type Provider interface {
	Run(*cache.Cache, types.CompletionParams, *providers.Options) ([]types.CompletionItem, error)
}

func NewManager(c *cache.Cache, opts *ManagerOptions) *Manager {
	return &Manager{c: c, opts: opts}
}

func NewDefaultManager(c *cache.Cache, store storage.Store) *Manager {
	m := NewManager(c, &ManagerOptions{})

	m.RegisterProvider(&providers.Package{})
	m.RegisterProvider(&providers.PackageName{})
	m.RegisterProvider(&providers.BuiltIns{})
	m.RegisterProvider(&providers.RegoV1{})
	m.RegisterProvider(&providers.PackageRefs{})
	m.RegisterProvider(&providers.RuleHead{})
	m.RegisterProvider(&providers.RuleHeadKeyword{})
	m.RegisterProvider(&providers.Input{})
	m.RegisterProvider(&providers.CommonRule{})
	m.RegisterProvider(&providers.UsedRefs{})

	m.RegisterProvider(providers.NewPolicy(store))

	return m
}

func (m *Manager) Run(params types.CompletionParams, opts *providers.Options) ([]types.CompletionItem, error) {
	var completions []types.CompletionItem

	if m.isInsideOfComment(params) {
		// Exit early if caret position is inside a comment. Most clients won't show
		// suggestions there anyway, and there's no need to ask providers for completions.
		return completions, nil
	}

	for _, provider := range m.providers {
		providerCompletions, err := provider.Run(m.c, params, opts)
		if err != nil {
			return nil, fmt.Errorf("error running completion provider: %w", err)
		}

		for _, completion := range providerCompletions {
			// if a provider returns a mandatory completion, return it immediately
			// as it is the only completion that should be shown.
			if completion.Mandatory {
				return []types.CompletionItem{completion}, nil
			}

			completions = append(completions, completion)
		}
	}

	return completions, nil
}

func (m *Manager) RegisterProvider(provider Provider) {
	m.providers = append(m.providers, provider)
}

func (m *Manager) isInsideOfComment(params types.CompletionParams) bool {
	if module, ok := m.c.GetModule(params.TextDocument.URI); ok {
		for _, comment := range module.Comments {
			cp := rego.PositionFromLocation(comment.Location)

			if cp.Line == params.Position.Line {
				if cp.Character <= params.Position.Character {
					return true
				}
			}
		}
	}

	return false
}
