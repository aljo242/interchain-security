package main

import (
	"time"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	gov "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	e2e "github.com/cosmos/interchain-security/v7/tests/e2e/testlib"
)

func stepStartProviderChain() []Step {
	return []Step{
		{
			Action: StartChainAction{
				Chain: ChainID("provi"),
				Validators: []StartChainValidator{
					{Id: ValidatorID("bob"), Stake: 500000000, Allocation: 10000000000},
					{Id: ValidatorID("alice"), Stake: 500000000, Allocation: 10000000000},
					{Id: ValidatorID("carol"), Stake: 500000000, Allocation: 10000000000},
				},
			},
			State: State{
				ChainID("provi"): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9500000000,
						ValidatorID("bob"):   9500000000,
						ValidatorID("carol"): 9500000000,
					},
				},
			},
		},
	}
}

func stepsStartPermissionlessChain(consumerName, consumerChainId string, proposedChains []string, validators []ValidatorID, chainIndex uint) []Step {
	s := []Step{
		{
			Action: CreateConsumerChainAction{
				Chain:         ChainID("provi"),
				From:          ValidatorID("alice"),
				ConsumerChain: ChainID(consumerName),
				InitParams: &InitializationParameters{
					InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
					SpawnTime:     uint(time.Minute * 3),
				},
				PowerShapingParams: &PowerShapingParameters{
					TopN: 0,
				},
			},
			State: State{
				ChainID("provi"): e2e.ChainState{
					ProposedConsumerChains: &proposedChains,
				},
			},
		},
	}

	// Assign validator keys
	// add a consumer key before the chain starts
	// the key will be present in the consumer genesis initial_val_set
	for _, valId := range validators {
		valCfg := getDefaultValidators()[valId]
		// no consumer-key assignment needed for validators using provider's public key
		if !valCfg.UseConsumerKey {
			continue
		}
		step := Step{
			Action: AssignConsumerPubKeyAction{
				Chain:          ChainID(consumerName),
				Validator:      valId,
				ConsumerPubkey: valCfg.ConsumerValPubKey,
				// consumer chain has not started
				// we don't need to reconfigure the node
				// since it will start with consumer key
				ReconfigureNode: false,
			},
			State: State{
				ChainID(consumerName): ChainState{
					AssignedKeys: &map[ValidatorID]string{
						valId: valCfg.ConsumerValconsAddressOnProvider,
					},
					ProviderKeys: &map[ValidatorID]string{
						valId: valCfg.ValconsAddress,
					},
				},
			},
		}
		s = append(s, step)
	}

	// Opt-in Validators
	for _, valId := range validators {
		step := Step{
			Action: OptInAction{
				Chain:     ChainID(consumerName),
				Validator: valId,
			},
			State: State{},
		}
		s = append(s, step)
	}

	// Launch chain
	step := Step{
		Action: UpdateConsumerChainAction{
			Chain:         ChainID("provi"),
			From:          ValidatorID("alice"),
			ConsumerChain: ChainID(consumerName),
			InitParams: &InitializationParameters{
				InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
				SpawnTime:     0, // launch now
			},
			PowerShapingParams: &PowerShapingParameters{
				TopN: 0,
			},
		},
		State: State{},
	}
	s = append(s, step)

	// Setup validators for  chain
	startChainVals := []StartChainValidator{}
	valBalance := map[ValidatorID]uint{
		ValidatorID("alice"): 0,
		ValidatorID("bob"):   0,
		ValidatorID("carol"): 0,
	}

	for idx, val := range validators {
		startChainVals = append(startChainVals,
			StartChainValidator{
				Id:         val,
				Stake:      uint(100000000 * (idx + 1)),
				Allocation: 10000000000,
			})
		valBalance[val] = 10000000000
	}

	// Start the chain
	step = Step{
		Action: StartConsumerChainAction{
			ConsumerChain: ChainID(consumerName),
			ProviderChain: ChainID("provi"),
			Validators:    startChainVals,
		},
		State: State{
			ChainID(consumerName): ChainState{
				ValBalances: &valBalance,
			},
		},
	}
	s = append(s, step)

	// Establish IBC connection
	steps := []Step{
		{
			Action: AddIbcConnectionAction{
				ChainA:  ChainID(consumerName),
				ChainB:  ChainID("provi"),
				ClientA: 0,
				ClientB: chainIndex,
			},
			State: State{},
		},
		{
			Action: AddIbcChannelAction{
				ChainA:      ChainID(consumerName),
				ChainB:      ChainID("provi"),
				ConnectionA: 0,
				PortA:       "consumer",
				PortB:       "provider",
				Order:       "ordered",
			},
			State: State{},
		},
	}
	s = append(s, steps...)
	return s
}

func stepsStartConsumerChain(consumerName string, proposalIndex, chainIndex uint, setupTransferChans bool) []Step {
	s := []Step{
		{
			Action: SubmitConsumerAdditionProposalAction{
				Chain:         ChainID("provi"),
				From:          ValidatorID("alice"),
				Deposit:       10000001,
				ConsumerChain: ChainID(consumerName),
				SpawnTime:     0,
				InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
				TopN:          100,
			},
			State: State{
				ChainID("provi"): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9489999999,
						ValidatorID("bob"):   9500000000,
					},
					Proposals: &map[uint]Proposal{
						proposalIndex: ConsumerAdditionProposal{
							Deposit:       10000001,
							Chain:         ChainID(consumerName),
							SpawnTime:     0,
							InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
							Status:        gov.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD.String(),
						},
					},
					ProposedConsumerChains: &[]string{consumerName},
				},
			},
		},
		// add a consumer key before the chain starts
		// the key will be present in consumer genesis initial_val_set
		{
			Action: AssignConsumerPubKeyAction{
				Chain:          ChainID(consumerName),
				Validator:      ValidatorID("carol"),
				ConsumerPubkey: getDefaultValidators()[ValidatorID("carol")].ConsumerValPubKey,
				// consumer chain has not started
				// we don't need to reconfigure the node
				// since it will start with consumer key
				ReconfigureNode: false,
			},
			State: State{
				ChainID(consumerName): ChainState{
					AssignedKeys: &map[ValidatorID]string{
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ConsumerValconsAddressOnProvider,
					},
					ProviderKeys: &map[ValidatorID]string{
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ValconsAddress,
					},
				},
			},
		},
		{
			// op should fail - key already assigned by the same validator
			Action: AssignConsumerPubKeyAction{
				Chain:           ChainID(consumerName),
				Validator:       ValidatorID("carol"),
				ConsumerPubkey:  getDefaultValidators()[ValidatorID("carol")].ConsumerValPubKey,
				ReconfigureNode: false,
				ExpectError:     true,
				ExpectedError:   "a validator has or had assigned this consumer key already",
			},
			State: State{},
		},
		{
			// op should fail - key already assigned by another validator
			Action: AssignConsumerPubKeyAction{
				Chain:     ChainID(consumerName),
				Validator: ValidatorID("bob"),
				// same pub key as carol
				ConsumerPubkey:  getDefaultValidators()[ValidatorID("carol")].ConsumerValPubKey,
				ReconfigureNode: false,
				ExpectError:     true,
				ExpectedError:   "a validator has or had assigned this consumer key already",
			},
			State: State{
				ChainID(consumerName): ChainState{
					AssignedKeys: &map[ValidatorID]string{
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ConsumerValconsAddressOnProvider,
						ValidatorID("bob"):   "",
					},
					ProviderKeys: &map[ValidatorID]string{
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ValconsAddress,
					},
				},
			},
		},
		{
			Action: VoteGovProposalAction{
				Chain:      ChainID("provi"),
				From:       []ValidatorID{ValidatorID("alice"), ValidatorID("bob"), ValidatorID("carol")},
				Vote:       []string{"yes", "yes", "yes"},
				PropNumber: proposalIndex,
			},
			State: State{
				ChainID("provi"): ChainState{
					Proposals: &map[uint]Proposal{
						proposalIndex: ConsumerAdditionProposal{
							Deposit:       10000001,
							Chain:         ChainID(consumerName),
							SpawnTime:     0,
							InitialHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 1},
							Status:        gov.ProposalStatus_PROPOSAL_STATUS_PASSED.String(),
						},
					},
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9500000000,
						ValidatorID("bob"):   9500000000,
					},
				},
			},
		},
		{
			Action: StartConsumerChainAction{
				ConsumerChain: ChainID(consumerName),
				ProviderChain: ChainID("provi"),
				Validators: []StartChainValidator{
					{Id: ValidatorID("bob"), Stake: 500000000, Allocation: 10000000000},
					{Id: ValidatorID("alice"), Stake: 500000000, Allocation: 10000000000},
					{Id: ValidatorID("carol"), Stake: 500000000, Allocation: 10000000000},
				},
			},
			State: State{
				ChainID("provi"): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 9500000000,
						ValidatorID("bob"):   9500000000,
						ValidatorID("carol"): 9500000000,
					},
					ProposedConsumerChains: &[]string{},
				},
				ChainID(consumerName): ChainState{
					ValBalances: &map[ValidatorID]uint{
						ValidatorID("alice"): 10000000000,
						ValidatorID("bob"):   10000000000,
						ValidatorID("carol"): 10000000000,
					},
				},
			},
		},
		{
			Action: AddIbcConnectionAction{
				ChainA:  ChainID(consumerName),
				ChainB:  ChainID("provi"),
				ClientA: 0,
				ClientB: chainIndex,
			},
			State: State{},
		},
		{
			Action: AddIbcChannelAction{
				ChainA:      ChainID(consumerName),
				ChainB:      ChainID("provi"),
				ConnectionA: 0,
				PortA:       "consumer", // TODO: check port mapping
				PortB:       "provider",
				Order:       "ordered",
			},
			State: State{},
		},
	}

	// currently only used in democracy tests
	if setupTransferChans {
		s = append(s, Step{
			Action: TransferChannelCompleteAction{
				ChainA:      ChainID(consumerName),
				ChainB:      ChainID("provi"),
				ConnectionA: 0,
				PortA:       "transfer",
				PortB:       "transfer",
				Order:       "unordered",
				ChannelA:    1,
				ChannelB:    1,
			},
			State: State{},
		})
	}
	return s
}

// starts provider and consumer chains specified in consumerNames
// setupTransferChans will establish a channel for fee transfers between consumer and provider
func stepsStartChains(consumerNames []string, setupTransferChans bool) []Step {
	s := stepStartProviderChain()
	for i, consumerName := range consumerNames {
		s = append(s, stepsStartConsumerChain(consumerName, uint(i+1), uint(i), setupTransferChans)...)
	}

	return s
}

func stepsAssignConsumerKeyOnStartedChain(consumerName, validator string) []Step {
	return []Step{
		{
			Action: AssignConsumerPubKeyAction{
				Chain:     ChainID(consumerName),
				Validator: ValidatorID("bob"),
				// reconfigure the node -> validator was using provider key
				// until this point -> key matches config.consumerValPubKey for "bob"
				ConsumerPubkey:  getDefaultValidators()[ValidatorID("bob")].ConsumerValPubKey,
				ReconfigureNode: true,
			},
			State: State{
				ChainID("provi"): ChainState{
					ValPowers: &map[ValidatorID]uint{
						// this happens after some delegations
						// so that the chain does not halt if 1/3 of power is offline
						ValidatorID("alice"): 511,
						ValidatorID("bob"):   500,
						ValidatorID("carol"): 500,
					},
				},
				ChainID(consumerName): ChainState{
					ValPowers: &map[ValidatorID]uint{
						// this happens after some delegations
						// so that the chain does not halt if 1/3 of power is offline
						ValidatorID("alice"): 511,
						ValidatorID("bob"):   500,
						ValidatorID("carol"): 500,
					},
					AssignedKeys: &map[ValidatorID]string{
						ValidatorID("bob"):   getDefaultValidators()[ValidatorID("bob")].ConsumerValconsAddressOnProvider,
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ConsumerValconsAddressOnProvider,
					},
					ProviderKeys: &map[ValidatorID]string{
						ValidatorID("bob"):   getDefaultValidators()[ValidatorID("bob")].ValconsAddress,
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ValconsAddress,
					},
				},
			},
		},
		{
			Action: RelayPacketsAction{
				ChainA:  ChainID("provi"),
				ChainB:  ChainID(consumerName),
				Port:    "provider",
				Channel: 0,
			},
			State: State{
				ChainID("provi"): ChainState{
					ValPowers: &map[ValidatorID]uint{
						// this happens after some delegations
						// so that the chain does not halt if 1/3 of power is offline
						ValidatorID("alice"): 511,
						ValidatorID("bob"):   500,
						ValidatorID("carol"): 500,
					},
				},
				ChainID(consumerName): ChainState{
					ValPowers: &map[ValidatorID]uint{
						// this happens after some delegations
						// so that the chain does not halt if 1/3 of power is offline
						ValidatorID("alice"): 511,
						ValidatorID("bob"):   500,
						ValidatorID("carol"): 500,
					},
					AssignedKeys: &map[ValidatorID]string{
						ValidatorID("bob"):   getDefaultValidators()[ValidatorID("bob")].ConsumerValconsAddressOnProvider,
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ConsumerValconsAddressOnProvider,
					},
					ProviderKeys: &map[ValidatorID]string{
						ValidatorID("bob"):   getDefaultValidators()[ValidatorID("bob")].ValconsAddress,
						ValidatorID("carol"): getDefaultValidators()[ValidatorID("carol")].ValconsAddress,
					},
				},
			},
		},
	}
}
