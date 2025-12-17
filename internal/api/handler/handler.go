package handler

import (
	"fmt"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/pkg/persistence"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/file"
	"go.uber.org/zap"
)

type Handler struct {
	SpoofRepo persistence.Repository[model.Spoof]
	log       *zap.Logger
}

func NewHandler() (*Handler, *zap.Logger, error) {
	h := &Handler{}
	/*
		logCfg := zap.NewProductionEncoderConfig()

		logCfg.TimeKey = "ts"
		logCfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006.01.02 15:04:05")
	*/
	switch config.GetInstance().Server().Env() {
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

		store, err := file.NewStore[model.Spoof]("store.json")
		if err != nil {
			return nil, hlog, fmt.Errorf("could not create filestore: %s", err.Error())
		}

		h.SpoofRepo = spoof.NewRepository(store)

	case "production", "prod", "PROD":

	}

	return h, h.log, nil
}
