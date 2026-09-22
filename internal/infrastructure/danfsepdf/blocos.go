package danfsepdf

import "github.com/edusouza/nfse-emissor-go/internal/domain/danfse"

// campo draws one cell: the dividing box, the label of item 2.4.2 in six points
// bold, and the content in seven points.
func (p *pagina) campo(rotulo, valor string, x, y, largura, altura float64) {
	p.caixa(x, y, largura, altura)
	p.escrever(rotulo, x+0.08, y+0.24, fonte, "B", corpoMiudo)
	p.escrever(valor, x+0.08, y+0.55, fonte, "", corpoTexto)
}

// titulo draws the name of a block in the leftmost cell of its first row,
// shaded as item 2.2.3 requires.
func (p *pagina) titulo(nome string, y float64) {
	p.quadroSombreado(colunaA, y, colunaLargura, blocoAltura)
	p.caixa(colunaA, y, colunaLargura, blocoAltura)
	p.escrever(nome, colunaA+0.08, y+0.42, fonte, "B", corpoBloco)
}

func (p *pagina) servico(s danfse.Servico) {
	p.titulo(tituloServico, servicoY)
	p.campo("Código de Tributação Nacional / Municipal", s.CodigoTributacao, colunaB, servicoY, colunaLargura, blocoAltura)
	p.campo("Código da NBS", s.CodigoNBS, colunaC, servicoY, colunaLargura, blocoAltura)
	p.campo("Local da Prestação / Sigla UF / País", s.LocalPrestacao, colunaD, servicoY, colunaLargura, blocoAltura)

	// The description of the taxation code is the one field the nota técnica
	// gives no label to, and its box is shorter than the others because of it.
	p.caixa(colunaA, descricaoCodigoY, larguraCorpo, descricaoCodigoH)
	p.escrever(s.DescricaoCodigo, colunaA+0.08, descricaoCodigoY+0.26, fonte, "", corpoTexto)

	p.campo("Descrição do Serviço", s.Descricao, colunaA, descricaoServicoY, larguraCorpo, blocoAltura)
}

func (p *pagina) issqn(i danfse.ISSQN) {
	if !i.Incide {
		// Note 4: an operation outside the municipal tax replaces the whole
		// block with one sentence.
		p.quadroSombreado(colunaA, issqnY, larguraCorpo, blocoAltura)
		p.caixa(colunaA, issqnY, larguraCorpo, blocoAltura)
		p.escrever(danfse.MensagemSemISSQN, colunaA+0.08, issqnY+0.42, fonte, "B", corpoBloco)
		return
	}

	p.titulo(tituloISSQN, issqnY)
	// The NT's table puts both this block's title and "Tipo de Tributação do
	// ISSQN" at 0,30 cm — the two cannot share a cell. The tax type moves one
	// column right, which is where every other block puts its first field, and
	// the municipality of incidence follows it with the double width the table
	// gives it. That fills the row exactly, and nothing overlaps.
	p.campo("Tipo de Tributação do ISSQN", i.TipoTributacao, colunaB, issqnY, colunaLargura, blocoAltura)
	p.campo("Município / Sigla UF / País da Incidência do ISSQN", i.MunicipioIncidencia, colunaC, issqnY, larguraDupla, blocoAltura)

	p.campo("Regime Especial de Tributação do ISSQN", i.RegimeEspecial, colunaA, issqnLinha2, colunaLargura, blocoAltura)
	p.campo("Tipo de Imunidade do ISSQN", i.TipoImunidade, colunaB, issqnLinha2, colunaLargura, blocoAltura)
	p.campo("Suspensão da Exigibilidade do ISSQN", i.SuspensaoExigibilidade, colunaC, issqnLinha2, colunaLargura, blocoAltura)
	p.campo("Número Processo Suspensão", i.NumeroProcessoSuspensao, colunaD, issqnLinha2, colunaLargura, blocoAltura)

	p.campo("Benefício Municipal", i.BeneficioMunicipal, colunaA, issqnLinha3, colunaLargura, blocoAltura)
	p.campo("Cálculo do BM", i.CalculoBM, colunaB, issqnLinha3, colunaLargura, blocoAltura)
	p.campo("Total Deduções/Reduções", i.TotalDeducoes, colunaC, issqnLinha3, colunaLargura, blocoAltura)
	p.campo("Desconto Incondicionado", i.DescontoIncondicionado, colunaD, issqnLinha3, colunaLargura, blocoAltura)

	p.campo("BC ISSQN", i.BaseCalculo, colunaA, issqnLinha4, colunaLargura, blocoAltura)
	p.campo("Alíquota Aplicada", i.Aliquota, colunaB, issqnLinha4, colunaLargura, blocoAltura)
	p.campo("Retenção do ISSQN", i.Retencao, colunaC, issqnLinha4, colunaLargura, blocoAltura)
	p.campo("ISSQN Apurado", i.Apurado, colunaD, issqnLinha4, colunaLargura, blocoAltura)
}

func (p *pagina) federal(f danfse.Federal) {
	p.titulo(tituloFederal, federalY)
	p.campo("IRRF", f.IRRF, colunaB, federalY, colunaLargura, blocoAltura)
	p.campo("Contribuição Previdenciária - Retida", f.ContribuicaoPrevidenc, colunaC, federalY, colunaLargura, blocoAltura)
	p.campo("Contribuições Sociais - Retidas", f.ContribuicoesSociais, colunaD, federalY, colunaLargura, blocoAltura)

	p.campo("PIS - Débito Apuração Própria", f.PIS, colunaA, federalLinha2, colunaLargura, blocoAltura)
	p.campo("COFINS - Débito Apuração Própria", f.COFINS, colunaB, federalLinha2, colunaLargura, blocoAltura)
	p.campo("Descrição Contrib. Sociais - Retidas", f.DescricaoContribuicoes, colunaC, federalLinha2, larguraDupla, blocoAltura)
}

func (p *pagina) ibscbs(i danfse.IBSCBS) {
	p.titulo(tituloIBSCBS, ibscbsY)
	p.campo("CST / cClassTrib", i.CST, colunaB, ibscbsY, colunaLargura, blocoAltura)
	p.campo("Indicador de Operação / Código IBGE Incidência / Município Incidência / Sigla UF",
		i.IndicadorOperacao, colunaC, ibscbsY, larguraDupla, blocoAltura)

	p.campo("Exclusões e Reduções da Base de Cálculo", i.ExclusoesReducoes, colunaA, ibscbsLinha2, colunaLargura, blocoAltura)
	p.campo("Base de Cálculo Após Exclusões e Reduções", i.BaseCalculo, colunaB, ibscbsLinha2, colunaLargura, blocoAltura)
	p.campo("Red. Alíquota IBS / Red. Alíquota CBS", i.ReducaoAliquota, colunaC, ibscbsLinha2, colunaLargura, blocoAltura)
	p.campo("Alíquota - IBS UF / IBS Mun", i.AliquotaIBS, colunaD, ibscbsLinha2, colunaLargura, blocoAltura)

	p.campo("Alíq. Efetiva Municipal - IBS", i.AliquotaEfetivaMun, colunaA, ibscbsLinha3, colunaLargura, blocoAltura)
	p.campo("Valor Apurado Municipal - IBS", i.ValorApuradoMun, colunaB, ibscbsLinha3, colunaLargura, blocoAltura)
	p.campo("Alíq. Efetiva Estadual - IBS", i.AliquotaEfetivaUF, colunaC, ibscbsLinha3, colunaLargura, blocoAltura)
	p.campo("Valor Apurado Estadual - IBS", i.ValorApuradoUF, colunaD, ibscbsLinha3, colunaLargura, blocoAltura)

	p.campo("Valor Total Apurado - IBS", i.ValorTotalIBS, colunaA, ibscbsLinha4, colunaLargura, blocoAltura)
	p.campo("Alíquota - CBS", i.AliquotaCBS, colunaB, ibscbsLinha4, colunaLargura, blocoAltura)
	p.campo("Alíquota Efetiva - CBS", i.AliquotaEfetivaCBS, colunaC, ibscbsLinha4, colunaLargura, blocoAltura)
	p.campo("Valor Total Apurado - CBS", i.ValorTotalCBS, colunaD, ibscbsLinha4, colunaLargura, blocoAltura)
}

func (p *pagina) totais(t danfse.Totais) {
	p.quadroSombreado(colunaA, totaisY, colunaLargura, totaisAltura)
	p.caixa(colunaA, totaisY, colunaLargura, totaisAltura)
	p.escrever(tituloTotais, colunaA+0.08, totaisY+0.44, fonte, "B", corpoBloco)

	p.campo("Valor da Operação / Serviço", t.ValorServico, colunaB, totaisY, colunaLargura, totaisAltura)
	p.campo("Desconto Incondicionado", t.DescontoIncondicionado, colunaC, totaisY, colunaLargura, totaisAltura)
	p.campo("Desconto Condicionado", t.DescontoCondicionado, colunaD, totaisY, colunaLargura, totaisAltura)

	p.campo("Total das Retenções (ISSQN / Federais)", t.TotalRetencoes, colunaA, totaisLinha2, colunaLargura, totaisAltura)
	p.campo("Valor Líquido da NFS-e", t.ValorLiquido, colunaB, totaisLinha2, colunaLargura, totaisAltura)
	p.campo("Total do IBS/CBS", t.TotalIBSCBS, colunaC, totaisLinha2, colunaLargura, totaisAltura)

	// Item 2.2.3 shades this field: it is the number the reader is looking for.
	p.quadroSombreado(colunaD, totaisLinha2, colunaLargura, totaisAltura)
	p.campo("Valor Líquido da NFS-e + IBS/CBS", t.ValorLiquidoComIBSCBS, colunaD, totaisLinha2, colunaLargura, totaisAltura)
}

// complementares fills the tall box of item 2.1.12, wrapping the joined text
// over as many lines as the box holds.
func (p *pagina) complementares(conteudo string, comCanhoto bool) {
	p.quadroSombreado(colunaA, complementaresTituloY, larguraCorpo, complementaresAltura)
	p.caixa(colunaA, complementaresTituloY, larguraCorpo, complementaresAltura)
	p.escrever(tituloComplementares, colunaA+0.08, complementaresTituloY+0.27, fonte, "B", corpoBloco)

	// Without the receipt strip the box may take the room it would have used —
	// item 2.3.3 says so in as many words.
	fim := canhotoY
	if !comCanhoto {
		fim = alturaPagina - margem
	}
	altura := fim - complementaresY
	p.caixa(colunaA, complementaresY, larguraCorpo, altura)

	p.pdf.SetFont(fonte, "", corpoTexto)
	linhas := p.pdf.SplitLines([]byte(p.traduzir(conteudo)), larguraCorpo-0.16)

	const entrelinha = 0.32
	for i, linha := range linhas {
		base := complementaresY + entrelinha*float64(i+1) - 0.08
		if base > complementaresY+altura {
			// The text is longer than the box. NT 008 already caps the field at
			// 2000 characters; stopping here keeps whatever is left from
			// running over the blocks below.
			break
		}
		p.pdf.Text(colunaA+0.08, base, string(linha))
	}
}

// canhoto draws the receipt strip of item 2.1.13.
//
// Two of its three fields have no source in the XML: the date of acknowledgement
// and the signature are written by hand on the printed page. They stay blank —
// the dash of note 12 belongs to a field the invoice left empty, not to a line
// meant for a pen.
func (p *pagina) canhoto(c danfse.Canhoto) {
	p.campo("Data de Cientificação", "", colunaA, canhotoY, colunaLargura, canhotoAltura)
	p.campo("Identificação e Assinatura", "", colunaB, canhotoY, colunaLargura, canhotoAltura)
	p.campo("Nº NFS-e / Chave NFS-e", c.Numero, colunaC, canhotoY, larguraDupla, canhotoAltura)
}

// marca writes the watermark of a cancelled or replaced invoice across the
// page, on the diagonal.
func (p *pagina) marca(marca danfse.Marca) {
	if marca == danfse.SemMarca {
		return
	}

	p.pdf.SetFont(fonte, "", marcaCorpo)
	p.pdf.SetTextColor(marcaCinza, marcaCinza, marcaCinza)

	texto := p.traduzir(string(marca))
	meioX, meioY := larguraPagina/2, alturaPagina/2

	p.pdf.TransformBegin()
	p.pdf.TransformRotate(marcaAngulo, meioX, meioY)
	p.pdf.Text(meioX-p.pdf.GetStringWidth(texto)/2, meioY, texto)
	p.pdf.TransformEnd()

	p.pdf.SetTextColor(0, 0, 0)
}

// pessoa draws one of the four blocks that name someone.
//
// A block with nobody in it collapses into the sentence notes 2 and 3 dictate,
// filling the rows it would have used: leaving four empty boxes on the page
// would suggest the invoice has fields nobody filled, when what it has is no
// such person.
func (p *pagina) pessoa(titulo string, dados danfse.Pessoa, linhas [3]float64, fim float64) {
	inicio := linhas[0]

	if dados.Mensagem != "" {
		altura := fim - inicio
		p.quadroSombreado(colunaA, inicio, larguraCorpo, altura)
		p.caixa(colunaA, inicio, larguraCorpo, altura)
		p.escrever(dados.Mensagem, colunaA+0.08, inicio+0.42, fonte, "B", corpoBloco)
		return
	}

	p.titulo(titulo, inicio)
	p.campo("CNPJ / CPF / NIF", dados.Documento, colunaB, inicio, colunaLargura, blocoAltura)
	if titulo == tituloDestinatario {
		// The recipient has no municipal registration in the leiaute; the
		// telephone takes the column it would occupy elsewhere.
		p.campo("Telefone", dados.Telefone, colunaD, inicio, colunaLargura, blocoAltura)
		p.caixa(colunaC, inicio, colunaLargura, blocoAltura)
	} else {
		p.campo("Indicador Municipal (Inscrição)", dados.InscricaoMunicipal, colunaC, inicio, colunaLargura, blocoAltura)
		p.campo("Telefone", dados.Telefone, colunaD, inicio, colunaLargura, blocoAltura)
	}

	linha2 := linhas[1]
	p.campo("Nome / Nome Empresarial", dados.Nome, colunaA, linha2, larguraDupla, blocoAltura)
	p.campo("Município / Sigla UF", dados.Municipio, colunaC, linha2, colunaLargura, blocoAltura)
	p.campo("Código IBGE / CEP", dados.CodigoCEP, colunaD, linha2, colunaLargura, blocoAltura)

	linha3 := linhas[2]
	p.campo("Endereço", dados.Endereco, colunaA, linha3, larguraDupla, blocoAltura)
	p.campo("E-mail", dados.Email, colunaC, linha3, larguraDupla, blocoAltura)
}

// prestador draws the block of item 2.1.3, which has one row more than the
// others: the Simples Nacional standing that decides how the invoice is taxed.
func (p *pagina) prestador(dados danfse.Prestador) {
	p.pessoa(tituloPrestador, dados.Pessoa,
		[3]float64{prestadorY, prestadorLinha2, prestadorLinha3}, tomadorY)

	p.campo("Simples Nacional na Data de Competência", dados.SimplesNacional,
		colunaA, prestadorLinha4, colunaLargura, blocoAltura)
	p.campo("Regime de Apuração Tributária pelo SN", dados.RegimeApuracao,
		colunaC, prestadorLinha4, larguraDupla, blocoAltura)
}
