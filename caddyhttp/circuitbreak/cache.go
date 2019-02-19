package circuitbreak

import (
	"errors"
	"github.com/cep21/circuit"
	"github.com/cep21/circuit/closers/hystrix"
	"time"
)

var (
	CacheCircuitIsOpenErr = errors.New("adminService circuit is open now!")
)

func NewCacheCircuit() *circuit.Circuit {
	return circuit.NewCircuitFromConfig("AdminCache",circuit.Config{
		General:circuit.GeneralConfig{
			OpenToClosedFactory:hystrix.CloserFactory(hystrix.ConfigureCloser{
				SleepWindow:30,
				RequiredConcurrentSuccessful:1,
			}),
			ClosedToOpenFactory:hystrix.OpenerFactory(hystrix.ConfigureOpener{
				RequestVolumeThreshold:1,
			}),
		},
		Execution:circuit.ExecutionConfig{
			Timeout:1 * time.Second,
		},

	})
}