package loaders

import "fmt"

type Loader interface {
	Load(dest any) error
}

// loads all the configuration in the following order. Where the last one "wins"
type ChainLoader struct {
	loaders []Loader
}

func NewChainLoader(loaders ...Loader) *ChainLoader {
	return &ChainLoader{loaders: loaders}
}

func (c *ChainLoader) Load(dest any) error {
	for _, loader := range c.loaders {
		if err := loader.Load(dest); err != nil {
			return fmt.Errorf("unable to load config: %s", err.Error())
		}
	}

	return nil
}
