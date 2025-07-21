package export

import (
	"context"
	"log"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func RunWithLeaderElection(onLeader func(ctx context.Context)) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Ошибка подключения к кластеру: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %v", err)
	}

	id, _ := os.Hostname()

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "testops-export-leader",
			Namespace: "default",
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onLeader,
			OnStoppedLeading: func() {
				log.Println("Я больше не лидер, останавливаю экспорт.")
				os.Exit(0)
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					log.Println("Я выбран лидером.")
				} else {
					log.Printf("Новый лидер: %s\n", identity)
				}
			},
		},
	})
}
