package app

import (
	"context"
	"fmt"
	"io"

	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
	"singbox-web/internal/templates"
)

type defaultBindingSource struct {
	templates *templates.Factory
	rules     *rules.Store
	random    io.Reader
}

func NewDefaultBindingSource(factory *templates.Factory, store *rules.Store, random io.Reader) *defaultBindingSource {
	return &defaultBindingSource{templates: factory, rules: store, random: random}
}
func (s *defaultBindingSource) DefaultBindings(ctx context.Context) ([]profile.RuleBinding, error) {
	ids, err := s.rules.EnsureTemplateResources(ctx, s.templates.ResourceTemplates(), s.random)
	if err != nil {
		return nil, err
	}
	source := s.templates.BindingTemplates()
	bindings := make([]profile.RuleBinding, 0, len(source))
	for _, item := range source {
		id, ok := ids[item.TemplateKey]
		if !ok {
			return nil, fmt.Errorf("默认规则模板缺少资源: %s", item.TemplateKey)
		}
		bindings = append(bindings, profile.RuleBinding{ResourceID: id, Tag: item.Tag, Enabled: item.Enabled})
	}
	return bindings, nil
}
