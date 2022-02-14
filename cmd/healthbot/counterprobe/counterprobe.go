package counterprobe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	jwtp "github.com/textileio/go-tableland/pkg/jwt"

	"github.com/ethereum/go-ethereum/rpc"
)

type CounterProbe struct {
	chckInterval time.Duration
	rpcClient    *rpc.Client
	ctrl         string
	tblname      string
}

func New(chckInterval time.Duration,
	endpoint, jwt, tblname string) (*CounterProbe, error) {
	if len(tblname) == 0 {
		return nil, errors.New("tablename is empty")
	}
	if _, err := url.ParseQuery(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint target: %s", err)
	}

	j, err := jwtp.Parse(jwt)
	if err != nil {
		return nil, fmt.Errorf("invalid jwt: %s", err)
	}
	if err := j.Verify(); err != nil {
		return nil, fmt.Errorf("validating jwt: %s", err)
	}
	if j.Claims.Issuer == "" {
		return nil, errors.New("jwt has no issuer")
	}

	rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("creating jsonrpc client: %s", err)
	}
	rpcClient.SetHeader("Authorization", "Bearer "+jwt)

	return &CounterProbe{
		chckInterval: chckInterval,
		rpcClient:    rpcClient,
		ctrl:         j.Claims.Issuer,
		tblname:      tblname,
	}, nil

}

func (cp *CounterProbe) Run(ctx context.Context) {
	log.Info().Msg("starting counter-probe...")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("closing gracefully...")
			return
		case <-time.After(cp.chckInterval):
			if err := cp.healthCheck(ctx); err != nil {
				log.Error().Err(err).Msg("health check failed")
			}
		}
	}
}

func (cp *CounterProbe) healthCheck(ctx context.Context) error {
	currentCounter, err := cp.getCurrentCounterValue(ctx)
	if err != nil {
		return fmt.Errorf("get current counter value: %s", err)
	}
	if err := cp.increaseCounterValue(ctx); err != nil {
		return fmt.Errorf("increasing counter value: %s", err)
	}
	updatedCounter, err := cp.getCurrentCounterValue(ctx)
	if err != nil {
		return fmt.Errorf("updated counter value: %s", err)
	}

	if updatedCounter != currentCounter+1 {
		return fmt.Errorf("unexpected updated counter value (exp: %d, got: %d)", currentCounter+1, updatedCounter)
	}

	return nil
}

func (cp *CounterProbe) increaseCounterValue(ctx context.Context) error {
	updateCounterReq := tableland.RunSQLRequest{
		Controller: cp.ctrl,
		Statement:  fmt.Sprintf("update %s set count=count+1", cp.tblname),
	}
	var updateCounterRes tableland.RunSQLResponse
	if err := cp.rpcClient.CallContext(ctx, &updateCounterRes, "tableland_runSQL", updateCounterReq); err != nil {
		return fmt.Errorf("calling tableland_runSQL: %s", err)
	}

	return nil
}

func (cp *CounterProbe) getCurrentCounterValue(ctx context.Context) (int, error) {
	getCounterReq := tableland.RunSQLRequest{
		Controller: cp.ctrl,
		Statement:  fmt.Sprintf("select * from %s", cp.tblname),
	}

	type Data struct {
		Rows [][]int `json:"rows"`
	}
	var getCounterRes struct {
		Result Data `json:"data"`
	}
	if err := cp.rpcClient.CallContext(ctx, &getCounterRes, "tableland_runSQL", getCounterReq); err != nil {
		return 0, fmt.Errorf("calling tableland_runSQL: %s", err)
	}
	if len(getCounterRes.Result.Rows) != 1 || len(getCounterRes.Result.Rows[0]) != 1 {
		return 0, fmt.Errorf("unexpected response format")
	}

	return getCounterRes.Result.Rows[0][0], nil
}
