package servicegroup

import "github.com/vitistack/gslb-operator/pkg/persistence"

func MigrateActiveToMap(defaultView string) persistence.MigrateFunc {
	return func(old map[string]any) (map[string]any, error) {
		active, ok := old["active"]
		if !ok {
			old["active"] = map[string]string{defaultView: ""}
			return old, nil
		}

		old["active"] = map[string]any{
			defaultView: active,
		}

		return old, nil
	}
}
