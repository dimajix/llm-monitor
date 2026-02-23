package interceptor

import (
	"context"
	"llm-monitor/internal/storage"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SavingInterceptor is a base struct for interceptors that save messages to storage
type SavingInterceptor struct {
	Name    string
	Storage storage.Storage
	Timeout time.Duration
}

// SaveToStorage saves the conversation history and assistant message to storage
func (si *SavingInterceptor) SaveToStorage(ctx context.Context, history []storage.SimpleMessage, assistantMsg storage.SimpleMessage, statusCode int, requestType string) {
	if si.Storage == nil {
		return
	}

	// 2. Try to find the deepest matching message ID
	var currentParentID uuid.UUID
	var currentBranchID uuid.UUID

	var curHistory = history
	for len(curHistory) > 0 {
		pid, err := si.Storage.FindMessageByHistory(ctx, curHistory, requestType)
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not find message by history", si.Name)
			return
		}
		if pid != uuid.Nil {
			// Do NOT create a new branch if the common messages actually is ONLY the first message AND its role is "system".
			// In such a case, a new conversation needs to be created instead.
			if len(curHistory) == 1 && curHistory[0].Role == "system" {
				currentParentID = uuid.Nil
				curHistory = curHistory[0:0]
			} else {
				currentParentID = pid
			}
			break
		}
		newLen := len(curHistory) - 1
		curHistory = curHistory[0:newLen]
		if newLen <= 0 {
			currentParentID = uuid.Nil
			break
		}
	}

	// Create new conversation if no message is found
	if currentParentID == uuid.Nil {
		// New conversation
		model := ""
		if len(history) > 0 {
			model = history[0].Model
		} else if assistantMsg.Model != "" {
			model = assistantMsg.Model
		}
		_, branch, err := si.Storage.CreateConversation(ctx, map[string]any{"model": model}, requestType)
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not create conversation in storage", si.Name)
			return
		}
		currentBranchID = branch.ID
	}

	// 3. Add missing messages from history
	for i, m := range history[len(curHistory):] {
		msg, err := si.Storage.AddMessage(ctx, currentParentID, &storage.Message{
			SimpleMessage: m,
			BranchID:      currentBranchID,
		})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not add history message %d to storage", si.Name, i)
			return
		}
		currentParentID = msg.ID
		currentBranchID = uuid.Nil // Only need it for the first message if no parent
	}

	// 4. Add the assistant response
	if assistantMsg.Content != "" || len(assistantMsg.ToolCalls) > 0 || statusCode != 0 {
		_, err := si.Storage.AddMessage(ctx, currentParentID, &storage.Message{
			SimpleMessage:      assistantMsg,
			UpstreamStatusCode: statusCode,
		})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not add assistant message to storage", si.Name)
		}
	}
}
