package nfe

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// =========================================================
// Constantes do WebService Distribuição DF-e
// =========================================================

const (
	VerDistDFe        = "1.01"
	xmlnsDistDFe      = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe"
	soapActionDistDFe = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe/nfeDistDFeInteresse"

	// URL fixa, não depende de UF
	urlDistDFe = "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
)

// =========================================================
// Estruturas do REQUEST
// =========================================================

type DistDFeInt struct {
	XMLName   xml.Name    `xml:"http://www.portalfiscal.inf.br/nfe distDFeInt"`
	Versao    string      `xml:"versao,attr"`
	TpAmb     TAmb        `xml:"tpAmb"`
	CUFAutor  string      `xml:"cUFAutor"`
	CNPJ      string      `xml:"CNPJ,omitempty"`
	CPF       string      `xml:"CPF,omitempty"`
	ConsChNFe *ConsChNFe  `xml:"consChNFe,omitempty"`
	// Pode adicionar ConsNSU futuramente
}

type ConsChNFe struct {
	ChNFe string `xml:"chNFe"`
}

// =========================================================
// Estruturas do RESPONSE
// =========================================================

type RetDistDFeInt struct {
	XMLName  xml.Name `xml:"retDistDFeInt"`
	Versao   string   `xml:"versao,attr"`
	TpAmb    TAmb     `xml:"tpAmb"`
	VerAplic string   `xml:"verAplic"`
	CStat    int      `xml:"cStat"`
	XMotivo  string   `xml:"xMotivo"`
	UltNSU   string   `xml:"ultNSU"`
	MaxNSU   string   `xml:"maxNSU"`

	Lote struct {
		Docs []DocZip `xml:"docZip"`
	} `xml:"loteDistDFeInt"`
}

type DocZip struct {
	Schema string `xml:"schema,attr"`
	Value  string `xml:",chardata"`
}

// =========================================================
// Função principal de consulta ao WebService
// =========================================================

func (req DistDFeInt) Consulta(client *http.Client, optReq ...func(req *http.Request)) (RetDistDFeInt, []byte, [][]byte, error) {
	optReq = append(optReq, func(r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		fmt.Println("===== SOAP REQUEST =====")
		fmt.Println(string(b))
		fmt.Println("========================")
		r.Body = io.NopCloser(bytes.NewReader(b)) // repõe body pro envio
})

	xmlfile, err := sendRequest(req, urlDistDFe, xmlnsDistDFe, soapActionDistDFe, client, optReq...)
	if err != nil {
		return RetDistDFeInt{}, nil, nil, fmt.Errorf("Erro na comunicação com a Sefaz Distribuição. Detalhes: %w", err)
	}

	var ret RetDistDFeInt
	if err := xml.Unmarshal(xmlfile, &ret); err != nil {
		return RetDistDFeInt{}, xmlfile, nil, fmt.Errorf("Erro ao desserializar o XML de retorno: %w. XML: %s", err, xmlfile)
	}

	// Decodificar automaticamente todos os docZip
	var docs [][]byte
	for _, doc := range ret.Lote.Docs {
		decoded, err := decodeDocZip(doc.Value)
		if err != nil {
			return ret, xmlfile, nil, fmt.Errorf("Erro ao decodificar docZip: %w", err)
		}
		docs = append(docs, decoded)
	}

	return ret, xmlfile, docs, nil
}

// =========================================================
// Função auxiliar: decodificar + descompactar docZip
// =========================================================

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

	xmlFinal, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}

	return xmlFinal, nil
}

// =========================================================
// Função utilitária simples (como ConsultaNFe)
// =========================================================

func ConsultaDistribuicaoPorChave(
	cnpj string,
	chave string,
	tpAmb TAmb,
	client *http.Client,
	optReq ...func(req *http.Request),
) (RetDistDFeInt, []byte, [][]byte, error) {

	// cUF = primeiros 2 dígitos da chave
	cUF := chave[:2]

	req := DistDFeInt{
		Versao:   VerDistDFe,
		TpAmb:    tpAmb,
		CUFAutor: cUF,
		CNPJ:     cnpj,
		ConsChNFe: &ConsChNFe{
			ChNFe: chave,
		},
	}

	return req.Consulta(client, optReq...)
}
