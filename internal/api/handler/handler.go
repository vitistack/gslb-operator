package handler

import (
	"fmt"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/memory"
	"go.uber.org/zap"
)

type Handler struct {
	spoofRepo persistence.Repository[model.Spoof]
	log       *zap.Logger
}

func NewHandler(cfg *config.Config) (*Handler, *zap.Logger, error) {
	h := &Handler{}
	/*
		logCfg := zap.NewProductionEncoderConfig()

		logCfg.TimeKey = "ts"
		logCfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006.01.02 15:04:05")
	*/
	switch cfg.Server.Environment {
	case "development", "dev", "DEV":
		hlog, err := zap.NewDevelopment(
			//zap.EncoderConfig(logCfg),
			zap.WithCaller(true),
			zap.AddStacktrace(zap.PanicLevel),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create handler: %s", err.Error())
		}
		h.log = hlog
		h.spoofRepo = spoof.NewRepository(memory.NewStore[model.Spoof]())

	case "production", "prod", "PROD":

	}

	return h, h.log, nil
}
