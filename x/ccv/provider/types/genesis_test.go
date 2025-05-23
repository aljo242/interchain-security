package types_test

import (
	"testing"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	abci "github.com/cometbft/cometbft/abci/types"
	tmtypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/interchain-security/v7/testutil/crypto"
	"github.com/cosmos/interchain-security/v7/x/ccv/provider/types"
	ccv "github.com/cosmos/interchain-security/v7/x/ccv/types"
)

// Tests validation of consumer states and params within a provider genesis state
func TestValidateGenesisState(t *testing.T) {
	testCases := []struct {
		name     string
		genState *types.GenesisState
		expPass  bool
	}{
		{
			"valid initializing provider genesis with nil updates",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-1", false)}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			true,
		},
		{
			"valid multiple provider genesis with multiple consumer chains",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{
					{ChainId: "chainid-1", ChannelId: "channelid1", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-1", false)},
					{ChainId: "chainid-2", ChannelId: "channelid2", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-2", true)},
					{ChainId: "chainid-3", ChannelId: "channelid3", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-3", false)},
					{ChainId: "chainid-4", ChannelId: "channelid4", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-4", true)},
				},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			true,
		},
		{
			"valid provider genesis with custom params",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-1", false)}},
				types.NewParams(types.DefaultTemplateClient(),
					types.DefaultTrustingPeriodFraction, time.Hour, time.Hour, "0.1", sdk.Coin{Denom: "stake", Amount: math.NewInt(10000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			true,
		},
		{
			"invalid zero valset update ID",
			types.NewGenesisState(
				0,
				nil,
				nil,
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid valset ID to block height mapping",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				[]types.ValsetUpdateIdToHeight{{ValsetUpdateId: 0}},
				nil,
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid params, zero trusting period fraction",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id"}},
				types.NewParams(types.DefaultTemplateClient(),
					"0.0", // 0 trusting period fraction here
					ccv.DefaultCCVTimeoutPeriod,
					types.DefaultSlashMeterReplenishPeriod,
					types.DefaultSlashMeterReplenishFraction,
					sdk.Coin{Denom: "stake", Amount: math.NewInt(10000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid params, zero ccv timeout",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id"}},
				types.NewParams(types.DefaultTemplateClient(),
					types.DefaultTrustingPeriodFraction,
					0, // 0 ccv timeout here
					types.DefaultSlashMeterReplenishPeriod,
					types.DefaultSlashMeterReplenishFraction,
					sdk.Coin{Denom: "stake", Amount: math.NewInt(1000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid params, zero slash meter replenish period",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id"}},
				types.NewParams(types.DefaultTemplateClient(),
					types.DefaultTrustingPeriodFraction,
					ccv.DefaultCCVTimeoutPeriod,
					0, // 0 slash meter replenish period here
					types.DefaultSlashMeterReplenishFraction,
					sdk.Coin{Denom: "stake", Amount: math.NewInt(10000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid params, invalid slash meter replenish fraction",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id"}},
				types.NewParams(types.DefaultTemplateClient(),
					types.DefaultTrustingPeriodFraction,
					ccv.DefaultCCVTimeoutPeriod,
					types.DefaultSlashMeterReplenishPeriod,
					"1.15",
					sdk.Coin{Denom: "stake", Amount: math.NewInt(10000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid consumer state channel id",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid", ChannelId: "invalidChannel{}", ClientId: "client-id"}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"empty consumer state client id",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid", ChannelId: "channel-0", ClientId: ""}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid consumer state client id 2",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid", ChannelId: "channel-0", ClientId: "abc", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid", false)}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid consumer state slash acks",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{
					ChainId: "chainid", ChannelId: "channel-0", ClientId: "client-id",
					ConsumerGenesis:  getInitialConsumerGenesis(t, "chainid", false),
					SlashDowntimeAck: []string{"cosmosvaloper1qlmk6r5w5taqrky4ycur4zq6jqxmuzr688htpp"},
				}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid consumer state pending VSC packets",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{
					ChainId: "chainid", ChannelId: "channel-0", ClientId: "client-id",
					ConsumerGenesis:      getInitialConsumerGenesis(t, "chainid", false),
					PendingValsetChanges: []ccv.ValidatorSetChangePacketData{{}},
				}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid consumer state pending VSC packets 2: invalid slash acks address",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{
					ChainId: "chainid", ChannelId: "channel-0", ClientId: "client-id",
					ConsumerGenesis: getInitialConsumerGenesis(t, "chainid", false),
					PendingValsetChanges: []ccv.ValidatorSetChangePacketData{{
						SlashAcks:        []string{"cosmosvaloper1qlmk6r5w5taqrky4ycur4zq6jqxmuzr688htpp"},
						ValsetUpdateId:   1,
						ValidatorUpdates: []abci.ValidatorUpdate{{}},
					}},
				}},
				types.DefaultParams(),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid params- invalid consumer registration fee denom",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-1", false)}},
				types.NewParams(types.DefaultTemplateClient(),
					types.DefaultTrustingPeriodFraction, time.Hour, time.Hour, "0.1", sdk.Coin{Denom: "st", Amount: math.NewInt(10000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			false,
		},
		{
			"invalid params- invalid consumer registration fee amount",
			types.NewGenesisState(
				types.DefaultValsetUpdateID,
				nil,
				[]types.ConsumerState{{ChainId: "chainid-1", ChannelId: "channelid", ClientId: "client-id", ConsumerGenesis: getInitialConsumerGenesis(t, "chainid-1", false)}},
				types.NewParams(types.DefaultTemplateClient(),
					types.DefaultTrustingPeriodFraction, time.Hour, time.Hour, "0.1", sdk.Coin{Denom: "stake", Amount: math.NewInt(-1000000)}, 600, 24, 180),
				nil,
				nil,
				nil,
			),
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genState.Validate()

			if tc.expPass {
				require.NoError(t, err, "test case: %s must pass", tc.name)
			} else {
				require.Error(t, err, "test case: %s must fail", tc.name)
			}
		})
	}
}

func getInitialConsumerGenesis(t *testing.T, chainID string, preCCV bool) ccv.ConsumerGenesisState {
	t.Helper()
	// generate validator public key
	cId := crypto.NewCryptoIdentityFromIntSeed(239668)
	pubKey := cId.TMCryptoPubKey()

	// create validator set with single validator
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	valHash := valSet.Hash()
	valUpdates := tmtypes.TM2PB.ValidatorUpdates(valSet)

	var clientState *ibctmtypes.ClientState = nil
	var consensusState *ibctmtypes.ConsensusState = nil
	connectionId := ""

	if preCCV {
		connectionId = "connection-1"
	} else {
		clientState = ibctmtypes.NewClientState(
			chainID,
			ibctmtypes.DefaultTrustLevel,
			time.Duration(1),
			time.Duration(2),
			time.Duration(1),
			clienttypes.Height{RevisionNumber: clienttypes.ParseChainID(chainID), RevisionHeight: 1},
			commitmenttypes.GetSDKSpecs(),
			[]string{"upgrade", "upgradedIBCState"})
		consensusState = ibctmtypes.NewConsensusState(time.Now(), commitmenttypes.NewMerkleRoot([]byte("apphash")), valHash)
	}

	params := ccv.DefaultParams()
	params.Enabled = true

	return *ccv.NewInitialConsumerGenesisState(clientState, consensusState, valUpdates, preCCV, connectionId, params)
}
