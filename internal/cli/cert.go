// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

// expiryWarningWindow is how close to expiry a certificate must be before the
// command flags it. An A1 certificate is valid for one year, so a month gives
// enough room to renew without disrupting emission.
const expiryWarningWindow = 30 * 24 * time.Hour

func newCertCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Inspeciona certificados digitais",
	}
	cmd.AddCommand(newCertInfoCommand())
	return cmd
}

func newCertInfoCommand() *cobra.Command {
	var (
		file     string
		password string
	)

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Mostra os dados de um certificado A1 (PFX/P12)",
		Long: `Le um certificado A1 no formato PFX/P12 e mostra titular, emissor,
validade e tamanho da chave, alem de apontar problemas que impediriam a
assinatura de uma DPS.

A senha pode vir de --senha, da variavel de ambiente ` + envCertPassword + `, ou
de um prompt interativo. Prefira a variavel de ambiente ou o prompt: argumentos
de linha de comando ficam visiveis na lista de processos do sistema.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pass, err := resolveCertPassword(password, cmd.Flags().Changed("senha"), cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("nao foi possivel ler o certificado: %w", err)
			}

			info, err := xmlsigner.ParsePFX(data, pass)
			if err != nil {
				return fmt.Errorf("nao foi possivel abrir o certificado (senha incorreta ou arquivo invalido): %w", err)
			}

			return printCertificate(cmd, info)
		},
	}

	cmd.Flags().StringVarP(&file, "arquivo", "a", "", "caminho do certificado PFX/P12 (obrigatorio)")
	cmd.Flags().StringVarP(&password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	_ = cmd.MarkFlagRequired("arquivo")

	return cmd
}

// printCertificate renders the certificate details, then its validation verdict.
func printCertificate(cmd *cobra.Command, info *xmlsigner.CertificateInfo) error {
	result := xmlsigner.NewCertificateValidator().ValidateWithDetails(info)
	out := cmd.OutOrStdout()

	if d := result.CertificateDetails; d != nil {
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "Titular\t%s\n", info.GetSubjectCN())
		fmt.Fprintf(tw, "Emissor\t%s\n", info.GetIssuerCN())
		fmt.Fprintf(tw, "Numero de serie\t%s\n", d.SerialNumber)
		fmt.Fprintf(tw, "Valido de\t%s\n", d.NotBefore.Local().Format("02/01/2006 15:04"))
		fmt.Fprintf(tw, "Valido ate\t%s\n", d.NotAfter.Local().Format("02/01/2006 15:04"))
		fmt.Fprintf(tw, "Dias restantes\t%d\n", d.DaysUntilExpiry)
		fmt.Fprintf(tw, "Tamanho da chave\t%d bits\n", d.KeySize)
		fmt.Fprintf(tw, "Certificados na cadeia\t%d\n", len(info.Chain))
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	fmt.Fprintln(out)

	for _, w := range result.Warnings {
		fmt.Fprintf(out, "aviso: %s\n", w)
	}

	if !result.Valid {
		for _, e := range result.Errors {
			fmt.Fprintf(out, "problema: %v\n", e)
		}
		return fmt.Errorf("o certificado nao pode ser usado para assinar uma DPS")
	}

	if d := result.CertificateDetails; d != nil && time.Until(d.NotAfter) < expiryWarningWindow {
		fmt.Fprintf(out, "aviso: o certificado expira em %d dia(s); providencie a renovacao\n", d.DaysUntilExpiry)
	}

	fmt.Fprintln(out, "Certificado apto a assinar uma DPS.")
	return nil
}
