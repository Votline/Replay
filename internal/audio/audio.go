package audio

import (
	"fmt"

	"go.uber.org/zap"

	gAu "github.com/Votline/Go-audio"
	gAcl "github.com/Votline/Go-audio/pkg/audio"
)

type AudioSegment struct {
	Start int64
	End   int64
}

type AudioClient struct {
	*gAcl.AudioClient
	log *zap.Logger
}

func Init(log *zap.Logger) (*AudioClient, error) {
	const op = "audio.Init"
	acl, err := gAu.InitAudioClient(0, 0, 0, 0, 0, 0, 0, 0, true, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if err := acl.AutoRouteMonitor(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &AudioClient{
		AudioClient: acl,
		log:         log,
	}, nil
}
