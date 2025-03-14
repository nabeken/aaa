package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type Command struct {
	Token       string `json:"token"`
	TeamID      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	ResponseURL string `json:"response_url"`

	// /letsencrypt
	Command string `json:"command"`

	// [command] [domain...]
	Text string `json:"text"`
}

func ParseCommand(payload []byte) (*Command, error) {
	command := &Command{}
	if err := json.Unmarshal(payload, command); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}

	for _, escaped := range []*string{
		&command.ResponseURL,
		&command.Command,
		&command.Text,
	} {
		unescaped, err := url.QueryUnescape(*escaped)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unescape")
		}

		*escaped = unescaped
	}

	return command, nil
}

type CommandResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

func PostResponse(respURL string, cmdResp *CommandResponse) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(cmdResp); err != nil {
		return errors.Wrap(err, "failed to encode to JSON")
	}

	resp, err := http.Post(respURL, "application/json", buf)
	if err != nil {
		return errors.Wrap(err, "failed to post the response to Slack")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.Errorf("failed to post the response to Slack: %s", string(respBody))
	}

	return nil
}

func PostErrorResponse(err error, slcmd *Command) error {
	resp := &CommandResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf("%s ERROR: `%s`", FormatUserName(slcmd.UserName), err),
	}

	return PostResponse(slcmd.ResponseURL, resp)
}

// https://api.slack.com/docs/message-formatting#linking_to_channels_and_users
func FormatUserName(name string) string {
	switch name {
	case "here", "channel":
		return "<!" + name + ">"
	}

	return "<@" + name + ">"
}
