package ollama

import (
	"context"
	"llm-monitor/internal/storage"

	"github.com/sirupsen/logrus"
)

func saveToStorage(ctx context.Context, s storage.Storage, name string, model string, history []storage.SimpleMessage, assistantMsg storage.SimpleMessage, statusCode int) {
	// 2. Try to find the deepest matching message ID
	var currentParentID string
	var currentBranchID string

	var curHistory = history
	for len(curHistory) > 0 {
		pid, err := s.FindMessageByHistory(ctx, curHistory)
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not find message by history", name)
			return
		}
		if pid != "" {
			currentParentID = pid
			break
		}
		newLen := len(curHistory) - 1
		curHistory = curHistory[0:newLen]
		if newLen <= 0 {
			currentParentID = ""
			break
		}
	}

	// Create new conversation if no message is found
	if currentParentID == "" {
		// New conversation
		_, branch, err := s.CreateConversation(ctx, map[string]any{"model": model})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not create conversation in storage", name)
			return
		}
		currentBranchID = branch.ID
	}

	// 3. Add missing messages from history
	for i, m := range history[len(curHistory):] {
		msg, err := s.AddMessage(ctx, currentParentID, &storage.Message{
			SimpleMessage: m,
			BranchID:      currentBranchID,
		})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not add history message %d to storage", name, i)
			return
		}
		currentParentID = msg.ID
		currentBranchID = "" // Only need it for the first message if no parent
	}

	// 4. Add the assistant response
	if assistantMsg.Content != "" || statusCode != 0 {
		_, err := s.AddMessage(ctx, currentParentID, &storage.Message{
			SimpleMessage:      assistantMsg,
			UpstreamStatusCode: statusCode,
		})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not add assistant message to storage", name)
		}
	}
}
