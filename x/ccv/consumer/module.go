package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"cosmossdk.io/core/appmodule"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/interchain-security/v7/x/ccv/consumer/client/cli"
	"github.com/cosmos/interchain-security/v7/x/ccv/consumer/keeper"
	consumertypes "github.com/cosmos/interchain-security/v7/x/ccv/consumer/types"
	ccvtypes "github.com/cosmos/interchain-security/v7/x/ccv/types"
)

var (
	_ module.AppModule           = (*AppModule)(nil)
	_ module.AppModuleBasic      = (*AppModuleBasic)(nil)
	_ module.AppModuleSimulation = (*AppModule)(nil)
	_ module.HasABCIGenesis      = (*AppModule)(nil)
	_ module.HasABCIEndBlock     = (*AppModule)(nil)
	_ module.HasName             = (*AppModule)(nil)
	_ module.HasConsensusVersion = (*AppModule)(nil)
	_ module.HasInvariants       = (*AppModule)(nil)
	_ module.HasServices         = (*AppModule)(nil)
	_ appmodule.AppModule        = (*AppModule)(nil)
	_ appmodule.HasBeginBlocker  = (*AppModule)(nil)
)

// AppModuleBasic is the IBC Consumer AppModuleBasic
type AppModuleBasic struct{}

// Name implements AppModuleBasic interface
func (AppModuleBasic) Name() string {
	return consumertypes.ModuleName
}

// RegisterLegacyAminoCodec implements AppModuleBasic interface
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	// ccv.RegisterLegacyAminoCodec(cdc)
}

// RegisterInterfaces registers module concrete types into protobuf Any.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	consumertypes.RegisterInterfaces(registry)
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

// DefaultGenesis returns default genesis state as raw bytes for the ibc
// consumer module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(ccvtypes.DefaultConsumerGenesisState())
}

// ValidateGenesis performs genesis state validation for the ibc consumer module.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var data consumertypes.GenesisState
	if err := cdc.UnmarshalJSON(bz, &data); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", consumertypes.ModuleName, err)
	}

	return data.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the ibc-consumer module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	err := consumertypes.RegisterQueryHandlerClient(context.Background(), mux, consumertypes.NewQueryClient(clientCtx))
	if err != nil {
		// same behavior as in cosmos-sdk
		panic(err)
	}
}

// GetTxCmd implements AppModuleBasic interface
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil
}

// GetQueryCmd implements AppModuleBasic interface
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.NewQueryCmd()
}

// AppModule represents the AppModule for this module
type AppModule struct {
	AppModuleBasic
	keeper     keeper.Keeper
	paramSpace paramtypes.Subspace
}

// NewAppModule creates a new consumer module
func NewAppModule(k keeper.Keeper, paramSpace paramtypes.Subspace) AppModule {
	return AppModule{
		keeper:     k,
		paramSpace: paramSpace,
	}
}

// RegisterInvariants implements the AppModule interface
func (AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	// TODO
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	consumertypes.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(&am.keeper))
	consumertypes.RegisterQueryServer(cfg.QueryServer(), am.keeper)

	m := keeper.NewMigrator(am.keeper, am.paramSpace)
	if err := cfg.RegisterMigration(consumertypes.ModuleName, 1, m.Migrate1to2); err != nil {
		panic(fmt.Sprintf("failed to register migrator for %s: %s", consumertypes.ModuleName, err))
	}
	if err := cfg.RegisterMigration(consumertypes.ModuleName, 2, m.Migrate2to3); err != nil {
		panic(fmt.Sprintf("failed to register migrator for %s: %s -- from 2 -> 3", consumertypes.ModuleName, err))
	}
	if err := cfg.RegisterMigration(consumertypes.ModuleName, 3, m.Migrate3to4); err != nil {
		panic(fmt.Sprintf("failed to register migrator for %s: %s -- from 3 -> 4", consumertypes.ModuleName, err))
	}
}

// InitGenesis performs genesis initialization for the consumer module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState consumertypes.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	return am.keeper.InitGenesis(ctx, &genesisState)
}

// ExportGenesis returns the exported genesis state as raw bytes for the consumer
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(gs)
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 {
	return 4
}

// BeginBlock implements the AppModule interface
// Set the VSC ID for the subsequent block to the same value as the current block
// Panic if the provider's channel was established and then closed
func (am AppModule) BeginBlock(goCtx context.Context) error {
	ctx := sdk.UnwrapSDKContext(goCtx)

	channelID, found := am.keeper.GetProviderChannel(ctx)
	if found && am.keeper.IsChannelClosed(ctx, channelID) {
		// The CCV channel was established, but it was then closed;
		// the consumer chain is not secured anymore, but we allow it to run as a POA chain and log an error.
		channelClosedMsg := fmt.Sprintf("CCV channel %q was closed - shutdown consumer chain since it is not secured anymore", channelID)
		am.keeper.Logger(ctx).Error(channelClosedMsg)
	}

	// map next block height to the vscID of the current block height
	blockHeight := uint64(ctx.BlockHeight())
	vID := am.keeper.GetHeightValsetUpdateID(ctx, blockHeight)
	am.keeper.SetHeightValsetUpdateID(ctx, blockHeight+1, vID)
	am.keeper.Logger(ctx).Debug("block height was mapped to vscID", "height", blockHeight+1, "vscID", vID)

	err := am.keeper.TrackHistoricalInfo(ctx)
	if err != nil {
		am.keeper.Logger(ctx).Warn("failed to track historical info", "error", err)
	}
	return nil
}

// EndBlock implements the AppModule interface
// Flush PendingChanges to ABCI, send pending packets, write acknowledgements for packets that have finished unbonding.
//
// TODO: e2e tests confirming behavior with and without standalone -> consumer changeover
func (am AppModule) EndBlock(goCtx context.Context) ([]abci.ValidatorUpdate, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// If PreCCV state is active, consumer is a previously standalone chain
	// that was just upgraded to include the consumer ccv module, execute changeover logic.
	if am.keeper.IsPreCCV(ctx) {
		initialValUpdates := am.keeper.ChangeoverToConsumer(ctx)
		return initialValUpdates, nil
	}

	// Execute EndBlock logic for the Reward Distribution sub-protocol
	am.keeper.EndBlockRD(ctx)

	// panics on invalid packets and unexpected send errors
	am.keeper.SendPackets(ctx)

	data, ok := am.keeper.GetPendingChanges(ctx)
	if !ok {
		return []abci.ValidatorUpdate{}, nil
	}
	// apply changes to cross-chain validator set
	tendermintUpdates := am.keeper.ApplyCCValidatorChanges(ctx, data.ValidatorUpdates)
	am.keeper.DeletePendingChanges(ctx)

	am.keeper.Logger(ctx).Debug("sending validator updates to consensus engine", "len updates", len(tendermintUpdates))

	return tendermintUpdates, nil
}

// AppModuleSimulation functions

// GenerateGenesisState creates a randomized GenState of the transfer module.
// TODO
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
}

// RegisterStoreDecoder registers a decoder for consumer module's types
// TODO
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
}

// WeightedOperations returns the all the consumer module operations with their respective weights.
func (am AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation {
	return nil
}
