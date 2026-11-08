// Package catalog traduz domínio → app → categoria. O catálogo embutido (catalog.yaml) é a
// base; as regras e os apps criados no app Android ficam no banco e valem por cima.
package catalog

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed catalog.yaml
var builtinYAML []byte

type Category struct {
	ID     string `yaml:"id" json:"id"`
	Label  string `yaml:"label" json:"label"`
	GapMin int    `yaml:"gap_min" json:"gap_min"`
	Ignore bool   `yaml:"ignore" json:"ignore"`
}

type App struct {
	ID       string   `yaml:"id" json:"id"`
	Name     string   `yaml:"name" json:"name"`
	Category string   `yaml:"category" json:"category"`
	Domains  []string `yaml:"domains" json:"domains"`
	Custom   bool     `yaml:"-" json:"custom"`
}

// Rule é uma classificação feita pelo usuário: domínio (e subdomínios) → app.
type Rule struct {
	Domain string `json:"domain"`
	AppID  string `json:"app_id"`
}

type Catalog struct {
	Categories []Category        `json:"categories"`
	Apps       []App             `json:"apps"`
	byDomain   map[string]string // sufixo → app
	apps       map[string]*App
	categories map[string]*Category
}

type file struct {
	Categories []Category `yaml:"categories"`
	Apps       []App      `yaml:"apps"`
}

// Builtin devolve só o catálogo embutido.
func Builtin() (*Catalog, error) { return Build(nil, nil) }

// Build junta o catálogo embutido com os apps e regras do usuário (que vencem).
func Build(custom []App, rules []Rule) (*Catalog, error) {
	var f file
	if err := yaml.Unmarshal(builtinYAML, &f); err != nil {
		return nil, fmt.Errorf("catálogo embutido: %w", err)
	}
	c := &Catalog{Categories: f.Categories, byDomain: map[string]string{}, apps: map[string]*App{}, categories: map[string]*Category{}}
	for i := range c.Categories {
		cat := &c.Categories[i]
		if cat.GapMin <= 0 {
			cat.GapMin = 5
		}
		c.categories[cat.ID] = cat
	}
	for _, a := range custom {
		a.Custom = true
		f.Apps = append(f.Apps, a)
	}
	for _, a := range f.Apps {
		if _, ok := c.categories[a.Category]; !ok {
			return nil, fmt.Errorf("app %s com categoria desconhecida %q", a.ID, a.Category)
		}
		if _, dup := c.apps[a.ID]; dup {
			return nil, fmt.Errorf("app repetido no catálogo: %s", a.ID)
		}
		c.Apps = append(c.Apps, a)
	}
	for i := range c.Apps {
		a := &c.Apps[i]
		c.apps[a.ID] = a
		for _, d := range a.Domains {
			c.byDomain[normalize(d)] = a.ID
		}
	}
	for _, r := range rules {
		if a, ok := c.apps[r.AppID]; ok {
			d := normalize(r.Domain)
			c.byDomain[d] = r.AppID
			if !contains(a.Domains, d) {
				a.Domains = append(a.Domains, d)
			}
		}
	}
	return c, nil
}

// Match devolve o app do domínio pelo sufixo mais longo; nil se não estiver no catálogo.
func (c *Catalog) Match(domain string) *App {
	d := normalize(domain)
	for {
		if id, ok := c.byDomain[d]; ok {
			return c.apps[id]
		}
		i := strings.IndexByte(d, '.')
		if i < 0 {
			return nil
		}
		d = d[i+1:]
	}
}

func (c *Catalog) App(id string) *App { return c.apps[id] }

func (c *Catalog) Category(id string) *Category { return c.categories[id] }

func normalize(d string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
