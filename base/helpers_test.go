package base

import (
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkBy(t *testing.T) {
	tests := []struct {
		name      string
		items     []int
		chunkSize int
		expected  [][]int
	}{
		{"exact multiple", []int{1, 2, 3, 4}, 2, [][]int{{1, 2}, {3, 4}}},
		{"remainder", []int{1, 2, 3, 4, 5}, 2, [][]int{{1, 2}, {3, 4}, {5}}},
		{"fewer items than chunk size", []int{1, 2}, 6, [][]int{{1, 2}}},
		{"chunk size 1", []int{1, 2, 3}, 1, [][]int{{1}, {2}, {3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, chunkBy(tt.items, tt.chunkSize))
		})
	}
}

type fakeCommandHandler struct {
	CommandHandler
	ephemeral bool
}

func (*fakeCommandHandler) GetCommands() []string       { return []string{"cmd"} }
func (h *fakeCommandHandler) GetScopes() []CommandScope { return commandScopes }
func (h *fakeCommandHandler) IsEphemeral() bool         { return h.ephemeral }

type plainCommandHandler struct{ CommandHandler }

func TestIsEphemeralCommand(t *testing.T) {
	assert.True(t, isEphemeralCommand(&fakeCommandHandler{ephemeral: true}, CommandScopeAllGroupChats))
	assert.True(t, isEphemeralCommand(&fakeCommandHandler{ephemeral: true}, CommandScopeAllChatAdmins))
	assert.False(t, isEphemeralCommand(&fakeCommandHandler{ephemeral: true}, CommandScopeAllPrivateChats))
	assert.False(t, isEphemeralCommand(&fakeCommandHandler{ephemeral: false}, CommandScopeAllGroupChats))
	assert.False(t, isEphemeralCommand(&plainCommandHandler{}, CommandScopeAllGroupChats))
}

func TestNewReply(t *testing.T) {
	chat := tgbotapi.Chat{ID: 100}
	regular := newReply(&tgbotapi.Message{MessageID: 5, Chat: chat, From: &tgbotapi.User{ID: 7}}, "text")
	assert.Equal(t, 5, regular.ReplyParameters.MessageID)
	assert.Zero(t, regular.ReplyParameters.EphemeralMessageID)
	assert.Zero(t, regular.EphemeralMessageParameters)

	ephemeral := newReply(&tgbotapi.Message{EphemeralMessageID: 9, Chat: chat, From: &tgbotapi.User{ID: 7}}, "text")
	assert.Zero(t, ephemeral.ReplyParameters.MessageID)
	assert.Equal(t, 9, ephemeral.ReplyParameters.EphemeralMessageID)
	assert.Equal(t, int64(7), ephemeral.EphemeralMessageParameters.ReceiverUserID)
}
