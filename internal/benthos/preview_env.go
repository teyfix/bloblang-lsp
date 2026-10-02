package benthos

import (
	"github.com/redpanda-data/benthos/v4/public/bloblang"
)

// PreviewEnvironment overrides env in a private IDE validation/execution environment.
// Production parsing, documentation, linting, and the process environment retain the real
// Benthos behavior. Imported mappings inherit this same preview environment.
func PreviewEnvironment(base *bloblang.Environment, sample *Sample) (*bloblang.Environment, error) {
	env := base.WithoutFunctions("env")
	spec := bloblang.NewPluginSpec().Param(bloblang.NewStringParam("name")).Impure()
	err := env.RegisterFunctionV2("env", spec, func(params *bloblang.ParsedParams) (bloblang.Function, error) {
		name, err := params.GetString("name")
		if err != nil {
			return nil, err
		}
		return func() (any, error) {
			if sample != nil {
				if value, ok := sample.Env[name]; ok {
					return value, nil
				}
			}
			return "", nil
		}, nil
	})
	return env, err
}
