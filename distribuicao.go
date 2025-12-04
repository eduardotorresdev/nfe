package nfe

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	xmlnsDistDFe      = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe"
	soapActionDistDFe = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe/nfeDistDFeInteresse"
	urlDistDFe        = "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
)

// ============================================================
// 1) distDFeInt (REQUEST)
// ============================================================

type DistDFeInt struct {
	XMLName   xml.Name  `xml:"http://www.portalfiscal.inf.br/nfe distDFeInt"`
	Versao    string    `xml:"versao,attr"`
	TpAmb     int       `xml:"tpAmb"`
	CUFAutor  int       `xml:"cUFAutor"`
	CNPJ      string    `xml:"CNPJ"`
	ConsChNFe ConsChNFe `xml:"consChNFe"`
}

type ConsChNFe struct {
	ChNFe string `xml:"chNFe"`
}

// ============================================================
// 2) Wrapper WSDL (BODY da requisição)
// ============================================================

type DistDFeWrapper struct {
	XMLName xml.Name `xml:"nfeDistDFeInteresse"`
	Xmlns   string   `xml:"xmlns,attr"`

	Dados struct {
		XMLName xml.Name   `xml:"nfeDadosMsg"`
		Xmlns   string     `xml:"xmlns,attr"`
		Msg     DistDFeInt `xml:"distDFeInt"`
	}
}

// ============================================================
// 3) Envelope SOAP 1.1 (REQUEST)
// ============================================================

type SoapEnvelopeDist struct {
	XMLName xml.Name     `xml:"soap:Envelope"`
	Soap    string       `xml:"xmlns:soap,attr"`
	Xsi     string       `xml:"xmlns:xsi,attr"`
	Xsd     string       `xml:"xmlns:xsd,attr"`
	Body    SoapBodyDist `xml:"soap:Body"`
}

type SoapBodyDist struct {
	Wrapper DistDFeWrapper `xml:"nfeDistDFeInteresse"`
}

// ============================================================
// 4) RESPONSE: retDistDFeInt + docZip (modelo bruto SEFAZ)
// ============================================================

type RetDistDFeInt struct {
	XMLName  xml.Name `xml:"retDistDFeInt"`
	Versao   string   `xml:"versao,attr"`
	TpAmb    int      `xml:"tpAmb"`
	VerAplic string   `xml:"verAplic"`
	CStat    int      `xml:"cStat"`
	XMotivo  string   `xml:"xMotivo"`
	DhResp   string   `xml:"dhResp"`
	UltNSU   string   `xml:"ultNSU"`
	MaxNSU   string   `xml:"maxNSU"`

	Lote struct {
		Docs []DocZip `xml:"docZip"`
	} `xml:"loteDistDFeInt"`
}

type DocZip struct {
	NSU    string `xml:"NSU,attr"`
	Schema string `xml:"schema,attr"`
	Value  string `xml:",chardata"`
}

// ============================================================
// 5) MODELO XML nfeProc (bruto, apenas para parse)
// ============================================================

type NFeProc struct {
	XMLName xml.Name `xml:"http://www.portalfiscal.inf.br/nfe nfeProc"`
	Versao  string   `xml:"versao,attr"`

	NFe     NFe     `xml:"NFe"`
	ProtNFe ProtNFe `xml:"protNFe"`
}

type NFe struct {
	InfNFe InfNFe `xml:"infNFe"`
}

type InfNFe struct {
	Id     string `xml:"Id,attr"`
	Versao string `xml:"versao,attr"`

	Ide     Ide      `xml:"ide"`
	Emit    Emit     `xml:"emit"`
	Dest    Dest     `xml:"dest"`
	Det     []Det    `xml:"det"`
	Total   Total    `xml:"total"`
	Transp  *Transp  `xml:"transp"`
	Cobr    *Cobr    `xml:"cobr"`
	Pag     *Pag     `xml:"pag"`
	InfAdic *InfAdic `xml:"infAdic"`
}

type Ide struct {
	CUF         int    `xml:"cUF"`
	CNF         string `xml:"cNF"`
	NatOp       string `xml:"natOp"`
	Mod         string `xml:"mod"`
	Serie       int    `xml:"serie"`
	NNF         int    `xml:"nNF"`
	DhEmi       string `xml:"dhEmi"`
	TpNF        int    `xml:"tpNF"`
	IdDest      int    `xml:"idDest"`
	CMunFG      int    `xml:"cMunFG"`
	TpImp       int    `xml:"tpImp"`
	TpEmis      int    `xml:"tpEmis"`
	CDV         int    `xml:"cDV"`
	TpAmb       int    `xml:"tpAmb"`
	FinNFe      int    `xml:"finNFe"`
	IndFinal    int    `xml:"indFinal"`
	IndPres     int    `xml:"indPres"`
	IndIntermed int    `xml:"indIntermed"`
	ProcEmi     int    `xml:"procEmi"`
	VerProc     string `xml:"verProc"`
}

type Emit struct {
	CNPJ      string `xml:"CNPJ"`
	XNome     string `xml:"xNome"`
	XFant     string `xml:"xFant"`
	EnderEmit Ender  `xml:"enderEmit"`
	IE        string `xml:"IE"`
	IM        string `xml:"IM"`
	CNAE      string `xml:"CNAE"`
	CRT       int    `xml:"CRT"`
}

type Dest struct {
	CNPJ      string `xml:"CNPJ"`
	XNome     string `xml:"xNome"`
	EnderDest Ender  `xml:"enderDest"`
	IndIEDest int    `xml:"indIEDest"`
	IE        string `xml:"IE"`
}

type Ender struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	CPais   string `xml:"cPais"`
	XPais   string `xml:"xPais"`
	Fone    string `xml:"fone"`
}

type Det struct {
	NItem     int     `xml:"nItem,attr"`
	Prod      Prod    `xml:"prod"`
	Imposto   Imposto `xml:"imposto"`
	InfAdProd string  `xml:"infAdProd"`
}

type Prod struct {
	CProd    string  `xml:"cProd"`
	CEAN     string  `xml:"cEAN"`
	XProd    string  `xml:"xProd"`
	NCM      string  `xml:"NCM"`
	CEST     string  `xml:"CEST"`
	CFOP     string  `xml:"CFOP"`
	UCom     string  `xml:"uCom"`
	QCom     float64 `xml:"qCom"`
	VUnCom   float64 `xml:"vUnCom"`
	VProd    float64 `xml:"vProd"`
	CEANTrib string  `xml:"cEANTrib"`
	UTrib    string  `xml:"uTrib"`
	QTrib    float64 `xml:"qTrib"`
	VUnTrib  float64 `xml:"vUnTrib"`
	IndTot   int     `xml:"indTot"`
	XPed     string  `xml:"xPed"`
	NFCI     string  `xml:"nFCI"`
}

type Imposto struct {
	VTotTrib float64 `xml:"vTotTrib"`

	ICMS struct {
		ICMS00 *ICMS00 `xml:"ICMS00"`
	} `xml:"ICMS"`

	IPI struct {
		CEnq    string   `xml:"cEnq"`
		IPITrib *IPITrib `xml:"IPITrib"`
		IPINT   *IPINT   `xml:"IPINT"`
	} `xml:"IPI"`

	PIS struct {
		PISAliq *PISAliq `xml:"PISAliq"`
	} `xml:"PIS"`

	COFINS struct {
		COFINSAliq *COFINSAliq `xml:"COFINSAliq"`
	} `xml:"COFINS"`
}

type ICMS00 struct {
	Orig  int     `xml:"orig"`
	CST   string  `xml:"CST"`
	ModBC int     `xml:"modBC"`
	VBC   float64 `xml:"vBC"`
	PICMS float64 `xml:"pICMS"`
	VICMS float64 `xml:"vICMS"`
}

type IPITrib struct {
	CST  string  `xml:"CST"`
	VBC  float64 `xml:"vBC"`
	PIPI float64 `xml:"pIPI"`
	VIPI float64 `xml:"vIPI"`
}

type IPINT struct {
	CST string `xml:"CST"`
}

type PISAliq struct {
	CST  string  `xml:"CST"`
	VBC  float64 `xml:"vBC"`
	PPIS float64 `xml:"pPIS"`
	VPIS float64 `xml:"vPIS"`
}

type COFINSAliq struct {
	CST     string  `xml:"CST"`
	VBC     float64 `xml:"vBC"`
	PCOFINS float64 `xml:"pCOFINS"`
	VCOFINS float64 `xml:"vCOFINS"`
}

type Total struct {
	ICMSTot ICMSTot `xml:"ICMSTot"`
}

type ICMSTot struct {
	VBC        float64 `xml:"vBC"`
	VICMS      float64 `xml:"vICMS"`
	VICMSDeson float64 `xml:"vICMSDeson"`
	VFCP       float64 `xml:"vFCP"`
	VBCST      float64 `xml:"vBCST"`
	VST        float64 `xml:"vST"`
	VFCPST     float64 `xml:"vFCPST"`
	VFCPSTRet  float64 `xml:"vFCPSTRet"`
	VProd      float64 `xml:"vProd"`
	VFrete     float64 `xml:"vFrete"`
	VSeg       float64 `xml:"vSeg"`
	VDesc      float64 `xml:"vDesc"`
	VII        float64 `xml:"vII"`
	VIPI       float64 `xml:"vIPI"`
	VIPIDevol  float64 `xml:"vIPIDevol"`
	VPIS       float64 `xml:"vPIS"`
	VCOFINS    float64 `xml:"vCOFINS"`
	VOutro     float64 `xml:"vOutro"`
	VNF        float64 `xml:"vNF"`
	VTotTrib   float64 `xml:"vTotTrib"`
}

type Transp struct {
	ModFrete   int         `xml:"modFrete"`
	Transporta *Transporta `xml:"transporta"`
	Vol        []Vol       `xml:"vol"`
}

type Transporta struct {
	CNPJ   string `xml:"CNPJ"`
	XNome  string `xml:"xNome"`
	IE     string `xml:"IE"`
	XEnder string `xml:"xEnder"`
	XMun   string `xml:"xMun"`
	UF     string `xml:"UF"`
}

type Vol struct {
	QVol  float64 `xml:"qVol"`
	Esp   string  `xml:"esp"`
	Marca string  `xml:"marca"`
	PesoL float64 `xml:"pesoL"`
	PesoB float64 `xml:"pesoB"`
}

type Cobr struct {
	Fat *Fat `xml:"fat"`
}

type Fat struct {
	NFat  string  `xml:"nFat"`
	VOrig float64 `xml:"vOrig"`
	VDesc float64 `xml:"vDesc"`
	VLiq  float64 `xml:"vLiq"`
}

type Pag struct {
	DetPag []DetPag `xml:"detPag"`
}

type DetPag struct {
	IndPag int     `xml:"indPag"`
	TPag   string  `xml:"tPag"`
	VPag   float64 `xml:"vPag"`
}

type InfAdic struct {
	InfCpl string `xml:"infCpl"`
}

type InfProt struct {
	Id       string    `xml:"Id,attr"`
	TpAmb    int       `xml:"tpAmb"`
	VerAplic string    `xml:"verAplic"`
	ChNFe    string    `xml:"chNFe"`
	DhRecbto time.Time `xml:"dhRecbto"`
	NProt    string    `xml:"nProt"`
	DigVal   string    `xml:"digVal"`
	CStat    int       `xml:"cStat"`
	XMotivo  string    `xml:"xMotivo"`
}

// ============================================================
// 6) MODELO SEMÂNTICO
// ============================================================

type ResultadoDistribuicaoNFe struct {
	Ambiente     int
	Aplicativo   string
	Status       int
	Motivo       string
	DataResposta time.Time
	UltimoNSU    string
	MaximoNSU    string

	Documentos []NotaFiscalDistribuida
}

type NotaFiscalDistribuida struct {
	NSU    string
	Schema string

	Chave            string
	Numero           int
	Serie            int
	Modelo           string
	NaturezaOperacao string
	DataEmissao      time.Time

	Emitente     ParteNFe
	Destinatario ParteNFe

	Itens  []ItemNotaFiscal
	Totais TotaisNotaFiscal

	Transporte *TransporteNotaFiscal
	Cobranca   *CobrancaNotaFiscal
	Pagamentos []PagamentoNotaFiscal
	Protocolo  ProtocoloNotaFiscal

	InformacoesComplementares string
}

type ParteNFe struct {
	CNPJ         string
	Nome         string
	NomeFantasia string
	IE           string
	Endereco     EnderecoNFe
}

type EnderecoNFe struct {
	Logradouro      string
	Numero          string
	Complemento     string
	Bairro          string
	Municipio       string
	CodigoMunicipio string
	UF              string
	CEP             string
	Pais            string
	CodigoPais      string
	Telefone        string
}

type ItemNotaFiscal struct {
	Numero        int
	Codigo        string
	CodigoEAN     string
	Descricao     string
	NCM           string
	CEST          string
	CFOP          string
	Unidade       string
	Quantidade    float64
	ValorUnitario float64
	ValorTotal    float64

	ValorTotalTributos float64

	ICMS   *ICMSItem
	IPI    *IPIItem
	PIS    *PISItem
	COFINS *COFINSItem

	Observacao string
}

type ICMSItem struct {
	Origem      int
	CST         string
	BaseCalculo float64
	Aliquota    float64
	Valor       float64
}

type IPIItem struct {
	CST                 string
	BaseCalculo         float64
	Aliquota            float64
	Valor               float64
	CodigoEnquadramento string
}

type PISItem struct {
	CST         string
	BaseCalculo float64
	Aliquota    float64
	Valor       float64
}

type COFINSItem struct {
	CST         string
	BaseCalculo float64
	Aliquota    float64
	Valor       float64
}

type TotaisNotaFiscal struct {
	ValorBaseICMS           float64
	ValorICMS               float64
	ValorProdutos           float64
	ValorNota               float64
	ValorTributosAproximado float64
}

type TransporteNotaFiscal struct {
	ModalidadeFrete   int
	Transportadora    *TransportadoraNotaFiscal
	QuantidadeVolumes float64
	Especie           string
	Marca             string
	PesoLiquido       float64
	PesoBruto         float64
}

type TransportadoraNotaFiscal struct {
	CNPJ      string
	Nome      string
	IE        string
	Endereco  string
	Municipio string
	UF        string
}

type CobrancaNotaFiscal struct {
	NumeroFatura  string
	ValorOriginal float64
	Desconto      float64
	ValorLiquido  float64
}

type PagamentoNotaFiscal struct {
	Indicador int
	Forma     string
	Valor     float64
}

type ProtocoloNotaFiscal struct {
	Numero          string
	DataRecebimento time.Time
	Status          int
	Motivo          string
}

// ============================================================
// sendRequestDist — SOAP 1.1 simples
// ============================================================

func sendRequestDist(soap []byte, url string, soapAction string, client *http.Client, optReq ...func(*http.Request)) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(soap))
	if err != nil {
		return nil, fmt.Errorf("erro criando request: %w", err)
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", soapAction)

	for _, fn := range optReq {
		fn(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ============================================================
// 7) Consulta DIST — retorna modelo SEMÂNTICO
// ============================================================

func ConsultaDistChNFe(
	cnpj string,
	chave string,
	tpAmb TAmb,
	client *http.Client,
	optReq ...func(*http.Request),
) (ResultadoDistribuicaoNFe, error) {
	// monta wrapper
	w := DistDFeWrapper{
		Xmlns: xmlnsDistDFe,
	}
	w.Dados.Xmlns = xmlnsDistDFe
	w.Dados.Msg = DistDFeInt{
		Versao:   "1.01",
		TpAmb:    int(tpAmb),
		CUFAutor: mustInt(chave[:2]),
		CNPJ:     cnpj,
		ConsChNFe: ConsChNFe{
			ChNFe: chave,
		},
	}

	// monta envelope SOAP
	env := SoapEnvelopeDist{
		Soap: "http://schemas.xmlsoap.org/soap/envelope/",
		Xsi:  "http://www.w3.org/2001/XMLSchema-instance",
		Xsd:  "http://www.w3.org/2001/XMLSchema",
		Body: SoapBodyDist{
			Wrapper: w,
		},
	}

	soapBody, err := xml.Marshal(env)
	if err != nil {
		return ResultadoDistribuicaoNFe{}, fmt.Errorf("erro marshal envelope SOAP: %w", err)
	}
	soapBody = append([]byte(xml.Header), soapBody...)

	// envia
	respSoap, err := sendRequestDist(
		soapBody,
		urlDistDFe,
		soapActionDistDFe,
		client,
		optReq...,
	)
	if err != nil {
		return ResultadoDistribuicaoNFe{}, err
	}

	// extrai <retDistDFeInt> de dentro do SOAP
	rawRet, err := extractRetDistDFeInt(respSoap)
	if err != nil {
		return ResultadoDistribuicaoNFe{}, err
	}

	var ret RetDistDFeInt
	if err := xml.Unmarshal(rawRet, &ret); err != nil {
		return ResultadoDistribuicaoNFe{}, fmt.Errorf("erro unmarshal retDistDFeInt: %w", err)
	}

	result := ResultadoDistribuicaoNFe{
		Ambiente:   ret.TpAmb,
		Aplicativo: ret.VerAplic,
		Status:     ret.CStat,
		Motivo:     strings.TrimSpace(ret.XMotivo),
		UltimoNSU:  strings.TrimSpace(ret.UltNSU),
		MaximoNSU:  strings.TrimSpace(ret.MaxNSU),
	}

	if ret.DhResp != "" {
		if t, err := time.Parse(time.RFC3339, ret.DhResp); err == nil {
			result.DataResposta = t
		}
	}

	for _, doc := range ret.Lote.Docs {
		// só tratamos NF-e autorizada (procNFe)
		if !strings.HasPrefix(doc.Schema, "procNFe") {
			continue
		}

		xmlDoc, err := decodeDocZip(doc.Value)
		if err != nil {
			return ResultadoDistribuicaoNFe{}, fmt.Errorf("erro decode docZip NSU=%s: %w", doc.NSU, err)
		}

		var proc NFeProc
		if err := xml.Unmarshal(xmlDoc, &proc); err != nil {
			return ResultadoDistribuicaoNFe{}, fmt.Errorf("erro unmarshal nfeProc NSU=%s: %w", doc.NSU, err)
		}

		nota, err := toNotaFiscalDistribuida(doc, proc)
		if err != nil {
			return ResultadoDistribuicaoNFe{}, fmt.Errorf("erro montar modelo semântico NSU=%s: %w", doc.NSU, err)
		}

		result.Documentos = append(result.Documentos, nota)
	}

	return result, nil
}

// ============================================================
// Helpers
// ============================================================

func mustInt(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}

// pega só o trecho <retDistDFeInt>...</retDistDFeInt> do SOAP
func extractRetDistDFeInt(soap []byte) ([]byte, error) {
	start := bytes.Index(soap, []byte("<retDistDFeInt"))
	if start == -1 {
		return nil, fmt.Errorf("tag <retDistDFeInt> não encontrada no SOAP")
	}

	end := bytes.Index(soap[start:], []byte("</retDistDFeInt>"))
	if end == -1 {
		return nil, fmt.Errorf("fechamento </retDistDFeInt> não encontrado no SOAP")
	}
	end += start + len("</retDistDFeInt>")

	return soap[start:end], nil
}

// base64 + gzip → XML original
func decodeDocZip(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	xmlBytes, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}

	return xmlBytes, nil
}

func toNotaFiscalDistribuida(doc DocZip, proc NFeProc) (NotaFiscalDistribuida, error) {
	inf := proc.NFe.InfNFe
	ide := inf.Ide
	emit := inf.Emit
	dest := inf.Dest

	nota := NotaFiscalDistribuida{
		NSU:    doc.NSU,
		Schema: doc.Schema,

		Numero:           ide.NNF,
		Serie:            ide.Serie,
		Modelo:           ide.Mod,
		NaturezaOperacao: strings.TrimSpace(ide.NatOp),

		Emitente: ParteNFe{
			CNPJ:         strings.TrimSpace(emit.CNPJ),
			Nome:         strings.TrimSpace(emit.XNome),
			NomeFantasia: strings.TrimSpace(emit.XFant),
			IE:           strings.TrimSpace(emit.IE),
			Endereco:     toEndereco(emit.EnderEmit),
		},
		Destinatario: ParteNFe{
			CNPJ:     strings.TrimSpace(dest.CNPJ),
			Nome:     strings.TrimSpace(dest.XNome),
			IE:       strings.TrimSpace(dest.IE),
			Endereco: toEndereco(dest.EnderDest),
		},
		Totais: TotaisNotaFiscal{
			ValorBaseICMS:           inf.Total.ICMSTot.VBC,
			ValorICMS:               inf.Total.ICMSTot.VICMS,
			ValorProdutos:           inf.Total.ICMSTot.VProd,
			ValorNota:               inf.Total.ICMSTot.VNF,
			ValorTributosAproximado: inf.Total.ICMSTot.VTotTrib,
		},
		Protocolo: ProtocoloNotaFiscal{
			Numero: strings.TrimSpace(proc.ProtNFe.InfProt.NProt),
			Status: proc.ProtNFe.InfProt.CStat,
			Motivo: strings.TrimSpace(proc.ProtNFe.InfProt.XMotivo),
		},
	}

	if ide.DhEmi != "" {
		if t, err := time.Parse(time.RFC3339, ide.DhEmi); err == nil {
			nota.DataEmissao = t
		}
	}

	nota.Chave = strings.TrimSpace(proc.ProtNFe.InfProt.ChNFe)
	if nota.Chave == "" && strings.HasPrefix(inf.Id, "NFe") {
		nota.Chave = strings.TrimPrefix(inf.Id, "NFe")
	}

	if !proc.ProtNFe.InfProt.DhRecbto.IsZero() {
		nota.Protocolo.DataRecebimento = proc.ProtNFe.InfProt.DhRecbto
	}

	if inf.InfAdic != nil {
		nota.InformacoesComplementares = strings.TrimSpace(inf.InfAdic.InfCpl)
	}

	if inf.Transp != nil {
		nota.Transporte = toTransporte(*inf.Transp)
	}

	if inf.Cobr != nil && inf.Cobr.Fat != nil {
		nota.Cobranca = &CobrancaNotaFiscal{
			NumeroFatura:  strings.TrimSpace(inf.Cobr.Fat.NFat),
			ValorOriginal: inf.Cobr.Fat.VOrig,
			Desconto:      inf.Cobr.Fat.VDesc,
			ValorLiquido:  inf.Cobr.Fat.VLiq,
		}
	}

	if inf.Pag != nil {
		for _, p := range inf.Pag.DetPag {
			nota.Pagamentos = append(nota.Pagamentos, PagamentoNotaFiscal{
				Indicador: p.IndPag,
				Forma:     p.TPag,
				Valor:     p.VPag,
			})
		}
	}

	for _, d := range inf.Det {
		item := ItemNotaFiscal{
			Numero:             d.NItem,
			Codigo:             strings.TrimSpace(d.Prod.CProd),
			CodigoEAN:          strings.TrimSpace(d.Prod.CEAN),
			Descricao:          strings.TrimSpace(d.Prod.XProd),
			NCM:                strings.TrimSpace(d.Prod.NCM),
			CEST:               strings.TrimSpace(d.Prod.CEST),
			CFOP:               strings.TrimSpace(d.Prod.CFOP),
			Unidade:            strings.TrimSpace(d.Prod.UCom),
			Quantidade:         d.Prod.QCom,
			ValorUnitario:      d.Prod.VUnCom,
			ValorTotal:         d.Prod.VProd,
			ValorTotalTributos: d.Imposto.VTotTrib,
			Observacao:         strings.TrimSpace(d.InfAdProd),
		}

		if d.Imposto.ICMS.ICMS00 != nil {
			ic := d.Imposto.ICMS.ICMS00
			item.ICMS = &ICMSItem{
				Origem:      ic.Orig,
				CST:         ic.CST,
				BaseCalculo: ic.VBC,
				Aliquota:    ic.PICMS,
				Valor:       ic.VICMS,
			}
		}

		if d.Imposto.IPI.IPITrib != nil {
			ip := d.Imposto.IPI.IPITrib
			item.IPI = &IPIItem{
				CST:                 ip.CST,
				BaseCalculo:         ip.VBC,
				Aliquota:            ip.PIPI,
				Valor:               ip.VIPI,
				CodigoEnquadramento: d.Imposto.IPI.CEnq,
			}
		} else if d.Imposto.IPI.IPINT != nil {
			item.IPI = &IPIItem{
				CST:                 d.Imposto.IPI.IPINT.CST,
				CodigoEnquadramento: d.Imposto.IPI.CEnq,
			}
		}

		if d.Imposto.PIS.PISAliq != nil {
			ps := d.Imposto.PIS.PISAliq
			item.PIS = &PISItem{
				CST:         ps.CST,
				BaseCalculo: ps.VBC,
				Aliquota:    ps.PPIS,
				Valor:       ps.VPIS,
			}
		}

		if d.Imposto.COFINS.COFINSAliq != nil {
			cf := d.Imposto.COFINS.COFINSAliq
			item.COFINS = &COFINSItem{
				CST:         cf.CST,
				BaseCalculo: cf.VBC,
				Aliquota:    cf.PCOFINS,
				Valor:       cf.VCOFINS,
			}
		}

		nota.Itens = append(nota.Itens, item)
	}

	return nota, nil
}

func toEndereco(e Ender) EnderecoNFe {
	return EnderecoNFe{
		Logradouro:      strings.TrimSpace(e.XLgr),
		Numero:          strings.TrimSpace(e.Nro),
		Complemento:     strings.TrimSpace(e.XCpl),
		Bairro:          strings.TrimSpace(e.XBairro),
		Municipio:       strings.TrimSpace(e.XMun),
		CodigoMunicipio: strings.TrimSpace(e.CMun),
		UF:              strings.TrimSpace(e.UF),
		CEP:             strings.TrimSpace(e.CEP),
		Pais:            strings.TrimSpace(e.XPais),
		CodigoPais:      strings.TrimSpace(e.CPais),
		Telefone:        strings.TrimSpace(e.Fone),
	}
}

func toTransporte(t Transp) *TransporteNotaFiscal {
	tr := &TransporteNotaFiscal{
		ModalidadeFrete: t.ModFrete,
	}

	if len(t.Vol) > 0 {
		v := t.Vol[0]
		tr.QuantidadeVolumes = v.QVol
		tr.Especie = strings.TrimSpace(v.Esp)
		tr.Marca = strings.TrimSpace(v.Marca)
		tr.PesoLiquido = v.PesoL
		tr.PesoBruto = v.PesoB
	}

	if t.Transporta != nil {
		tr.Transportadora = &TransportadoraNotaFiscal{
			CNPJ:      strings.TrimSpace(t.Transporta.CNPJ),
			Nome:      strings.TrimSpace(t.Transporta.XNome),
			IE:        strings.TrimSpace(t.Transporta.IE),
			Endereco:  strings.TrimSpace(t.Transporta.XEnder),
			Municipio: strings.TrimSpace(t.Transporta.XMun),
			UF:        strings.TrimSpace(t.Transporta.UF),
		}
	}

	return tr
}
