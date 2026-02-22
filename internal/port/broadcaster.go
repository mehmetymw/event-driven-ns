package port

type StatusBroadcaster interface {
	Broadcast(notificationID string, status string, timestamp string)
}
