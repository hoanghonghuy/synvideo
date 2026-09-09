package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

var errRuntimeToolchainUnavailable = errors.New("runtime toolchain unavailable")

type runtimeToolchainResponse struct {
	FFmpegVersion  string `json:"ffmpeg_version"`
	FFprobeVersion string `json:"ffprobe_version"`
	ProfileID      string `json:"profile_id,omitempty"`
}

type runtimeToolchainProbe func() (runtimeToolchainResponse, error)

func runtimeToolchainHandler(probe runtimeToolchainProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		response, err := probe()
		if err != nil {
			writeProjectJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "unavailable"})
			return
		}
		writeProjectJSON(w, http.StatusOK, response)
	}
}

func probeRuntimeToolchain() (runtimeToolchainResponse, error) {
	ctx := context.Background()
	profile, err := renderexport.ProbeLocalFFmpegProfile(ctx, nil)
	if err != nil {
		return runtimeToolchainResponse{}, errRuntimeToolchainUnavailable
	}
	runner := renderexport.ExecCommandRunner{}
	ffprobeOutput, err := runner.Run(ctx, renderexport.FFprobeBinary, "-hide_banner", "-version")
	if err != nil {
		return runtimeToolchainResponse{}, errRuntimeToolchainUnavailable
	}
	ffprobeVersion := firstNonEmptyLine(string(ffprobeOutput))
	if ffprobeVersion == "" {
		return runtimeToolchainResponse{}, errRuntimeToolchainUnavailable
	}
	return runtimeToolchainResponse{
		FFmpegVersion:  profile.VersionLine,
		FFprobeVersion: ffprobeVersion,
		ProfileID:      profile.ID,
	}, nil
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
