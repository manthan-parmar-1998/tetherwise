package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// ParsePlanFile reads a `terraform show -json` plan from path.
func ParsePlanFile(path string) ([]model.Resource, []model.Dependency, []model.Change, error) {
	f, err := os.Open(path) //nolint:gosec // G304: operator-selected plan file
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open terraform plan: %w", err)
	}
	defer func() { _ = f.Close() }()
	resources, deps, changes, err := ParsePlan(f)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse terraform plan %s: %w", path, err)
	}
	return resources, deps, changes, nil
}

// ParseStateFile reads a `terraform show -json` state from path.
func ParseStateFile(path string) ([]model.Resource, []model.Dependency, error) {
	f, err := os.Open(path) //nolint:gosec // G304: operator-selected state file
	if err != nil {
		return nil, nil, fmt.Errorf("open terraform state: %w", err)
	}
	defer func() { _ = f.Close() }()
	resources, deps, err := ParseState(f)
	if err != nil {
		return nil, nil, fmt.Errorf("parse terraform state %s: %w", path, err)
	}
	return resources, deps, nil
}

// ParsePlan converts a plan document into resources, dependencies, and changes.
func ParsePlan(r io.Reader) ([]model.Resource, []model.Dependency, []model.Change, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read plan: %w", err)
	}
	var plan tfjson.Plan
	if err := json.Unmarshal(buf, &plan); err != nil {
		return nil, nil, nil, fmt.Errorf("decode plan: %w", err)
	}
	b := newBuilder()
	if plan.PriorState != nil && plan.PriorState.Values != nil {
		b.absorbModule(plan.PriorState.Values.RootModule)
	}
	if plan.PlannedValues != nil {
		b.absorbModule(plan.PlannedValues.RootModule)
	}
	b.linkDepends()
	if plan.Config != nil {
		b.walkConfig(plan.Config.RootModule, "")
	}
	b.matchValues()
	changes, err := b.changes(plan.ResourceChanges)
	if err != nil {
		return nil, nil, nil, err
	}
	resources, deps := b.done()
	return resources, deps, changes, nil
}

// ParseState converts a state document into resources and dependencies.
func ParseState(r io.Reader) ([]model.Resource, []model.Dependency, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read state: %w", err)
	}
	var state tfjson.State
	if err := json.Unmarshal(buf, &state); err != nil {
		return nil, nil, fmt.Errorf("decode state: %w", err)
	}
	b := newBuilder()
	if state.Values != nil {
		b.absorbModule(state.Values.RootModule)
	}
	b.linkDepends()
	b.matchValues()
	resources, deps := b.done()
	return resources, deps, nil
}

type tfResource struct {
	res       model.Resource
	typ       string
	address   string
	values    map[string]any
	dependsOn []string
}

type builder struct {
	byAddr map[string]*tfResource
	byID   map[string]*tfResource
	deps   map[string]model.Dependency
}

func newBuilder() *builder {
	return &builder{
		byAddr: map[string]*tfResource{},
		byID:   map[string]*tfResource{},
		deps:   map[string]model.Dependency{},
	}
}

func (b *builder) absorbModule(mod *tfjson.StateModule) {
	if mod == nil {
		return
	}
	for _, res := range mod.Resources {
		b.absorbStateResource(res)
	}
	children := append([]*tfjson.StateModule(nil), mod.ChildModules...)
	sort.Slice(children, func(i, j int) bool { return children[i].Address < children[j].Address })
	for _, child := range children {
		b.absorbModule(child)
	}
}

func (b *builder) absorbStateResource(res *tfjson.StateResource) {
	if res == nil || res.Mode == tfjson.DataResourceMode {
		return
	}
	values := res.AttributeValues
	if values == nil {
		values = map[string]any{}
	}
	hidden := sensitiveKeys(res.SensitiveValues)
	built, ok := identify(res.Type, res.Address, values, hidden)
	if !ok {
		return
	}
	item := &tfResource{
		res: built, typ: res.Type, address: res.Address, values: values,
		dependsOn: append([]string(nil), res.DependsOn...),
	}
	b.byAddr[res.Address] = item
	b.byID[built.ID] = item
}

func (b *builder) linkDepends() {
	addrs := make([]string, 0, len(b.byAddr))
	for addr := range b.byAddr {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	for _, addr := range addrs {
		item := b.byAddr[addr]
		for _, dep := range item.dependsOn {
			target := b.byAddr[dep]
			if target == nil {
				continue
			}
			b.add(item.res.ID, target.res.ID, relation(item.typ, "", target.res.Kind), model.ConfidenceHigh)
		}
	}
}

func (b *builder) walkConfig(mod *tfjson.ConfigModule, prefix string) {
	if mod == nil {
		return
	}
	resources := append([]*tfjson.ConfigResource(nil), mod.Resources...)
	sort.Slice(resources, func(i, j int) bool { return resources[i].Address < resources[j].Address })
	for _, res := range resources {
		if res.Mode == tfjson.DataResourceMode {
			continue
		}
		addr := joinAddr(prefix, res.Address)
		from := b.byAddr[addr]
		if from == nil {
			continue
		}
		attrs := make([]string, 0, len(res.Expressions))
		for attr := range res.Expressions {
			attrs = append(attrs, attr)
		}
		sort.Strings(attrs)
		for _, attr := range attrs {
			b.walkExpr(from, attr, res.Expressions[attr])
		}
		for _, dep := range res.DependsOn {
			b.linkRef(from, "", qualify(prefix, dep), model.ConfidenceHigh)
		}
	}
	names := make([]string, 0, len(mod.ModuleCalls))
	for name := range mod.ModuleCalls {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		call := mod.ModuleCalls[name]
		if call == nil {
			continue
		}
		next := joinAddr(prefix, "module."+name)
		b.walkConfig(call.Module, next)
	}
}

func (b *builder) walkExpr(from *tfResource, attr string, expr *tfjson.Expression) {
	if expr == nil || expr.ExpressionData == nil {
		return
	}
	refs := append([]string(nil), expr.References...)
	sort.Strings(refs)
	for _, ref := range refs {
		b.linkRef(from, attr, ref, model.ConfidenceHigh)
	}
	for _, block := range expr.NestedBlocks {
		keys := make([]string, 0, len(block))
		for k := range block {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			childAttr := attr
			if childAttr != "" {
				childAttr += "."
			}
			childAttr += k
			b.walkExpr(from, childAttr, block[k])
		}
	}
}

func (b *builder) linkRef(from *tfResource, attr, ref string, conf model.Confidence) {
	target := b.lookup(ref)
	if target == nil {
		return
	}
	b.add(from.res.ID, target.res.ID, relation(from.typ, attr, target.res.Kind), conf)
}

func (b *builder) lookup(ref string) *tfResource {
	best := ""
	for addr := range b.byAddr {
		if ref == addr || strings.HasPrefix(ref, addr+".") {
			if len(addr) > len(best) {
				best = addr
			}
		}
	}
	if best == "" {
		return nil
	}
	return b.byAddr[best]
}

func (b *builder) matchValues() {
	endpoints := map[string]string{}
	groups := map[string]*tfResource{}
	for _, item := range b.byID {
		for _, key := range []string{"endpoint", "domain_endpoint", "address", "bootstrap_brokers", "bootstrap_brokers_tls", "dns_name"} {
			if ep := endpointValue(item.values, key); len(ep) >= 8 {
				endpoints[ep] = item.res.ID
			}
		}
		if item.res.Kind == "ec2:security-group" {
			if id := item.res.Attrs["id"]; id != "" {
				groups[id] = item
			}
			if name := item.res.Attrs["name"]; name != "" {
				groups[name] = item
			}
		}
	}
	ids := make([]string, 0, len(b.byID))
	for id := range b.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	eps := make([]string, 0, len(endpoints))
	for ep := range endpoints {
		eps = append(eps, ep)
	}
	sort.Strings(eps)
	for _, id := range ids {
		item := b.byID[id]
		for _, text := range collectStrings(item.values) {
			for _, ep := range eps {
				owner := endpoints[ep]
				if owner == id || !strings.Contains(text, ep) {
					continue
				}
				target := b.byID[owner]
				if target == nil {
					continue
				}
				b.add(id, owner, relation(item.typ, "endpoint", target.res.Kind), model.ConfidenceMedium)
			}
		}
		for _, sg := range securityGroupRefs(item.values) {
			target := groups[sg]
			if target == nil || target.res.ID == id {
				continue
			}
			b.add(id, target.res.ID, model.KindConnectsTo, model.ConfidenceMedium)
		}
	}
}

func (b *builder) add(from, to, kind string, conf model.Confidence) {
	if from == "" || to == "" || from == to || kind == "" {
		return
	}
	dep := model.Dependency{
		From: from, To: to, Kind: kind,
		Source:     model.SourceTerraform,
		Sources:    []string{model.SourceTerraform},
		Confidence: conf,
	}
	if err := dep.Validate(); err != nil {
		return
	}
	key := from + "\x00" + to + "\x00" + kind
	prev, ok := b.deps[key]
	if !ok || conf.Rank() > prev.Confidence.Rank() {
		b.deps[key] = dep
	}
}

func (b *builder) changes(list []*tfjson.ResourceChange) ([]model.Change, error) {
	var out []model.Change
	for _, rc := range list {
		if rc == nil || rc.Change == nil || rc.Mode == tfjson.DataResourceMode {
			continue
		}
		action, ok := changeAction(rc.Change.Actions)
		if !ok {
			continue
		}
		item := b.byAddr[rc.Address]
		var res model.Resource
		if item != nil {
			res = item.res
		} else {
			values := asMap(rc.Change.After)
			if values == nil {
				values = asMap(rc.Change.Before)
			}
			built, identified := identify(rc.Type, rc.Address, values, nil)
			if !identified {
				continue
			}
			res = built
		}
		attrs := changedAttrs(rc.Change.Before, rc.Change.After, rc.Change.BeforeSensitive, rc.Change.AfterSensitive)
		ch := model.Change{Resource: res.ID, Action: action, ChangedAttrs: attrs}
		if err := ch.Validate(); err != nil {
			return nil, fmt.Errorf("change %s: %w", rc.Address, err)
		}
		out = append(out, ch)
	}
	model.SortChanges(out)
	return out, nil
}

func (b *builder) done() ([]model.Resource, []model.Dependency) {
	resources := make([]model.Resource, 0, len(b.byID))
	for _, item := range b.byID {
		resources = append(resources, item.res)
	}
	model.SortResources(resources)
	deps := make([]model.Dependency, 0, len(b.deps))
	for _, dep := range b.deps {
		deps = append(deps, dep)
	}
	model.SortDependencies(deps)
	return resources, deps
}

func changeAction(actions tfjson.Actions) (model.Action, bool) {
	switch {
	case actions.Replace():
		return model.ActionReplace, true
	case actions.Create():
		return model.ActionCreate, true
	case actions.Update():
		return model.ActionUpdate, true
	case actions.Delete():
		return model.ActionDelete, true
	default:
		return "", false
	}
}

func relation(fromType, attr, targetKind string) string {
	attr = strings.ToLower(attr)
	logShipper := fromType == "aws_cloudwatch_log_subscription_filter" || fromType == "aws_kinesis_firehose_delivery_stream"
	if logShipper && strings.Contains(attr, "log_group") {
		return model.KindReadsFrom
	}
	if logShipper && (targetKind == "opensearch:domain" || strings.Contains(attr, "destination") || strings.Contains(attr, "endpoint") || attr == "") {
		if targetKind == "logs:log-group" {
			return model.KindReadsFrom
		}
		return model.KindWritesLogsTo
	}
	if strings.Contains(attr, "security_group") {
		return model.KindConnectsTo
	}
	if strings.Contains(attr, "read") {
		return model.KindReadsFrom
	}
	if strings.Contains(attr, "log") || strings.Contains(attr, "index") {
		return model.KindWritesLogsTo
	}
	return model.KindConnectsTo
}

func joinAddr(prefix, addr string) string {
	if prefix == "" {
		return addr
	}
	if addr == "" {
		return prefix
	}
	return prefix + "." + addr
}

func qualify(prefix, ref string) string {
	if prefix == "" || strings.HasPrefix(ref, "module.") {
		return ref
	}
	return prefix + "." + ref
}
