package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/spf13/cobra"

	"github.com/cosmos/ibc-go/v3/modules/apps/29-fee/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

const (
	flagRecvFee    = "recv-fee"
	flagAckFee     = "ack-fee"
	flagTimeoutFee = "timeout-fee"
)

// NewPayPacketFeeAsyncTxCmd returns the command to create a MsgPayPacketFeeAsync
func NewPayPacketFeeAsyncTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pay-packet-fee [src-port] [src-channel] [sequence]",
		Short:   "Pay a fee to incentivize an existing IBC packet",
		Long:    strings.TrimSpace(`Pay a fee to incentivize an existing IBC packet.`),
		Example: fmt.Sprintf("tx %s pay-packet-fee transfer channel-0 1 --recv-fee 10stake --ack-fee 10stake --timeout-fee 10stake", version.AppName),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// NOTE: specifying non-nil relayers is currently unsupported
			var relayers []string

			sender := clientCtx.GetFromAddress().String()
			seq, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return err
			}

			packetID := channeltypes.NewPacketId(args[1], args[0], seq)

			recvFeeStr, err := cmd.Flags().GetString(flagRecvFee)
			if err != nil {
				return err
			}

			recvFee, err := sdk.ParseCoinsNormalized(recvFeeStr)
			if err != nil {
				return err
			}

			ackFeeStr, err := cmd.Flags().GetString(flagAckFee)
			if err != nil {
				return err
			}

			ackFee, err := sdk.ParseCoinsNormalized(ackFeeStr)
			if err != nil {
				return err
			}

			timeoutFeeStr, err := cmd.Flags().GetString(flagTimeoutFee)
			if err != nil {
				return err
			}

			timeoutFee, err := sdk.ParseCoinsNormalized(timeoutFeeStr)
			if err != nil {
				return err
			}

			fee := types.Fee{
				RecvFee:    recvFee,
				AckFee:     ackFee,
				TimeoutFee: timeoutFee,
			}

			identifiedPacketFee := types.NewIdentifiedPacketFee(packetID, fee, sender, relayers)

			msg := types.NewMsgPayPacketFeeAsync(identifiedPacketFee)
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String(flagRecvFee, "", "Fee paid to a relayer for relaying a packet receive.")
	cmd.Flags().String(flagAckFee, "", "Fee paid to a relayer for relaying a packet acknowledgement.")
	cmd.Flags().String(flagTimeoutFee, "", "Fee paid to a relayer for relaying a packet timeout.")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewRegisterCounterpartyAddress returns the command to create a MsgRegisterCounterpartyAddress
func NewRegisterCounterpartyAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "register-counter-party [address] [counterparty-address] [channel-id]",
		Short:   "Register a counterparty relayer address on a given channel.",
		Long:    strings.TrimSpace(`Register a counterparty relayer address on a given channel.`),
		Example: fmt.Sprintf("tx %s register-counter-party cosmos1rsp837a4kvtgp2m4uqzdge0zzu6efqgucm0qdh cosmoss1sp921a4tttgpln6rqhdqe0zzu6efqgucm0qdh channel-0", version.AppName),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(args[0], args[1], args[2])

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgRegisterCounterpartyAddress(args[0], args[1], args[2])

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
