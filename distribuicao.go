package nfe

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

const (
	xmlnsDistDFe      = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe"
	soapActionDistDFe = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe/nfeDistDFeInteresse"
	urlDistDFe        = "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
)

// ============================================================
// 1) distDFeInt
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
// 2) Wrapper WSDL (Body)
// ============================================================

type DistDFeWrapper struct {
	XMLName xml.Name `xml:"nfeDistDFeInteresse"`
	Xmlns   string   `xml:"xmlns,attr"`

	Dados struct {
		XMLName xml.Name  `xml:"nfeDadosMsg"`
		Xmlns   string    `xml:"xmlns,attr"`
		Msg     DistDFeInt `xml:"distDFeInt"`
	}
}

// ============================================================
// 2.1) Envelope SOAP 1.1
// ============================================================

type SoapEnvelopeDist struct {
	XMLName xml.Name      `xml:"soap:Envelope"`
	Soap    string        `xml:"xmlns:soap,attr"`
	Xsi     string        `xml:"xmlns:xsi,attr"`
	Xsd     string        `xml:"xmlns:xsd,attr"`
	Body    SoapBodyDist  `xml:"soap:Body"`
}

type SoapBodyDist struct {
	Wrapper DistDFeWrapper `xml:"nfeDistDFeInteresse"`
}

// ============================================================
// sendRequestDist — simples, SOAP 1.1
// ============================================================

func sendRequestDist(soap []byte, url string, soapAction string, client *http.Client, optReq ...func(*http.Request)) ([]byte, error) {

	req, err := http.NewRequest("POST", url, bytes.NewReader(soap))
	if err != nil {
		return nil, fmt.Errorf("erro criando request: %w", err)
	}

	// SOAP 1.1
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
// 3) Consulta DIST sem tratar retorno
// ============================================================

func ConsultaDistChNFe(cnpj string, chave string, tpAmb TAmb, client *http.Client, optReq ...func(*http.Request)) error {

	// monta body nfeDistDFeInteresse / nfeDadosMsg / distDFeInt
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

	// envelope SOAP 1.1
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
		return fmt.Errorf("erro marshal envelope SOAP: %w", err)
	}

	soapBody = append([]byte(xml.Header), soapBody...)

	respSoap, err := sendRequestDist(
		soapBody,
		urlDistDFe,
		soapActionDistDFe,
		client,
		optReq...,
	)
	if err != nil {
		fmt.Println("Erro na distribuição:", err)
		return err
	}

	fmt.Println("Distribuição OK")
	fmt.Println("Resposta SOAP bruta:")
	fmt.Println(string(respSoap))

	return nil
}

// ============================================================
// Helper
// ============================================================

func mustInt(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}
