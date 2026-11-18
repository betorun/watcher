package kubernetes

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var incidentNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func podWith(statuses ...corev1.ContainerStatus) *corev1.Pod {
	return &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: statuses}}
}

func running() corev1.ContainerState {
	return corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(incidentNow)}}
}

func oomAt(at time.Time) corev1.ContainerState {
	return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reasonOOMKilled, ExitCode: 137,
		FinishedAt: metav1.NewTime(at)}}
}

func TestPodIncident(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want string
	}{
		{"sain", podWith(corev1.ContainerStatus{Name: "app", State: running()}), ""},
		{"expulsé", &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, Reason: reasonEvicted,
			Message: "The node was low on resource: memory."}}, reasonEvicted},
		{"crashloop simple", podWith(corev1.ContainerStatus{Name: "app",
			State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonCrashLoopBackOff}},
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1}}}),
			reasonCrashLoopBackOff},
		// La cause (mémoire) dit au client quoi faire ; CrashLoopBackOff, non.
		{"crashloop causé par la mémoire", podWith(corev1.ContainerStatus{Name: "app",
			State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonCrashLoopBackOff}},
			LastTerminationState: oomAt(incidentNow.Add(-10 * time.Minute))}),
			reasonOOMKilled},
		{"redémarré après OOM, récent", podWith(corev1.ContainerStatus{Name: "app", State: running(), RestartCount: 1,
			LastTerminationState: oomAt(incidentNow.Add(-30 * time.Second))}), reasonOOMKilled},
		// Ancien arrêt : une resynchronisation ne doit pas rouvrir l'incident.
		{"redémarré après OOM, ancien", podWith(corev1.ContainerStatus{Name: "app", State: running(), RestartCount: 1,
			LastTerminationState: oomAt(incidentNow.Add(-time.Hour))}), ""},
		{"arrêté en erreur", podWith(corev1.ContainerStatus{Name: "app",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 2}}}), "Error"},
		{"terminé normalement", podWith(corev1.ContainerStatus{Name: "app",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reasonCompleted}}}), ""},
		{"conteneur d'init en échec", &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{{Name: "migrate",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reasonCrashLoopBackOff}}}}}},
			reasonCrashLoopBackOff},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _, _ := podIncident(c.pod, incidentNow); got != c.want {
				t.Fatalf("raison %q, attendu %q", got, c.want)
			}
		})
	}
}
