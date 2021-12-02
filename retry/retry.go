package retry

type Policy interface {
}

func New() *Retry {
	return &Retry{
		Config: &Config{
			// Policies: new(TimeoutHandler).SetTimeout(time.Second * 5),
		},
	}
}

type Config struct {
	Policies Policy
}

type Retry struct {
	*Config
}

func (c *Retry) SetConfig(cfg *Config) *Retry {
	c.Config = cfg
	return c
}

func (c *Retry) Run(fn func() (interface{}, error)) (interface{}, error) {

	return nil, nil
}
