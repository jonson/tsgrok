package funnel

type FunnelRegistry struct {
	Funnels map[string]Funnel
}

func (f *FunnelRegistry) AddFunnel(funnel Funnel) {
	f.Funnels[funnel.HTTPFunnel.id] = funnel
}

func (f *FunnelRegistry) RemoveFunnel(id string) {
	delete(f.Funnels, id)
}

func (f *FunnelRegistry) GetFunnel(id string) (Funnel, error) {
	return f.Funnels[id], nil
}

func NewFunnelRegistry() *FunnelRegistry {
	return &FunnelRegistry{
		Funnels: make(map[string]Funnel),
	}
}
