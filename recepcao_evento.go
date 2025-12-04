// recepcao_evento.go
package nfe

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1" // assinatura rsa-sha1
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// ============================================================================
// Constantes da Recepção de Evento
// ============================================================================

const (
	xmlnsRecepcaoEvento      = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4"
	xmlnsNFe                 = "http://www.portalfiscal.inf.br/nfe"
	urlRecepcaoEvento        = "https://www.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx"
	soapActionRecepcaoEvento = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4/nfeRecepcaoEventoNF"
)

// ============================================================================
// Modelo de entrada (Manifestação)
// ============================================================================

type ManifestacaoEvento struct {
	COrgao     int
	TpAmb      int
	CNPJ       string
	CPF        string
	ChNFe      string
	DhEvento   time.Time
	TpEvento   string
	NSeqEvento int
	VerEvento  string
	DescEvento string
}

// ============================================================================
// Modelo de saída semântica
// ============================================================================

type ManifestacaoEventoItem struct {
	Ambiente          int       // tpAmb
	CodigoOrgao       int       // cOrgao
	CodigoStatus      int       // cStat
	Motivo            string    // xMotivo
	ChNFe             string    // chNFe
	TipoEvento        string    // tpEvento
	DescricaoEvento   string    // xEvento
	NumeroSequencial  int       // nSeqEvento
	DataRegistro      time.Time // dhRegEvento
	VersaoAplicativo  string    // verAplic
}

type ManifestacaoEventoResponse struct {
	LoteID           string                   // idLote
	Ambiente         int                      // tpAmb
	CodigoOrgao      int                      // cOrgao
	CodigoStatus     int                      // cStat
	Motivo           string                   // xMotivo
	VersaoAplicativo string                   // verAplic
	Eventos          []ManifestacaoEventoItem // retEvento[..].infEvento
}

// ============================================================================
// Função pública: envia Manifestação de Evento
// ============================================================================

func SendManifestacaoEvento(
	ctx context.Context,
	client *http.Client,
	certPEMPath, keyPEMPath string,
	idLote string,
	eventos []ManifestacaoEvento,
	optReq ...func(*http.Request),
) (*ManifestacaoEventoResponse, []byte, error) {
	if len(eventos) == 0 {
		return nil, nil, fmt.Errorf("nenhum evento informado")
	}

	// Carrega cert/key (PEM) para assinatura
	certPEM, keyPEM, err := loadSigningCert(certPEMPath, keyPEMPath)
	if err != nil {
		return nil, nil, fmt.Errorf("erro carregando cert/key: %w", err)
	}

	// 1) Monta envEvento (Document) já assinado
	envDoc, err := buildEnvEventoDoc(idLote, eventos, certPEM, keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("erro montando envEvento: %w", err)
	}

	envXML, _ := envDoc.WriteToString()
	fmt.Println("==== envEvento XML ====")
	fmt.Println(envXML)
	fmt.Println("=======================")

	// 2) Monta SOAP 1.2 com envEvento dentro de <nfeDadosMsg>
	soapDoc := etree.NewDocument()
	env := soapDoc.CreateElement("soap12:Envelope")
	env.CreateAttr("xmlns:soap12", "http://www.w3.org/2003/05/soap-envelope")
	env.CreateAttr("xmlns:xsi", "http://www.w3.org/2001/XMLSchema-instance")
	env.CreateAttr("xmlns:xsd", "http://www.w3.org/2001/XMLSchema")

	body := env.CreateElement("soap12:Body")
	nfeDadosMsg := body.CreateElement("nfeDadosMsg")
	nfeDadosMsg.CreateAttr("xmlns", xmlnsRecepcaoEvento)

	// importa o root do envEvento doc
	nfeDadosMsg.AddChild(envDoc.Root())

	soapDoc.SetRoot(env)
	soapBytes, err := soapDoc.WriteToBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("erro gerando SOAP XML: %w", err)
	}

	fmt.Println("==== SOAP ENVIADO RecepcaoEvento ====")
	fmt.Println(string(soapBytes))
	fmt.Println("=====================================")

	// 3) Envia com o SEU http.Client
	req, err := http.NewRequestWithContext(ctx, "POST", urlRecepcaoEvento, bytes.NewReader(soapBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("erro criando request HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", soapActionRecepcaoEvento)

	for _, fn := range optReq {
		fn(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("erro na requisição HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	fmt.Println("==== SOAP RESPOSTA RecepcaoEvento ====")
	fmt.Println(string(respBody))
	fmt.Println("======================================")

	if resp.StatusCode != http.StatusOK {
		return nil, respBody, fmt.Errorf("HTTP %d na RecepcaoEvento", resp.StatusCode)
	}

	sem, err := parseManifestacaoResp(respBody)
	if err != nil {
		return nil, respBody, fmt.Errorf("erro parse resposta RecepcaoEvento: %w", err)
	}

	return sem, respBody, nil
}

// ============================================================================
// Montagem do envEvento (Document) + assinatura manual do infEvento
// ============================================================================

func buildEnvEventoDoc(
	idLote string,
	eventos []ManifestacaoEvento,
	certPEM, keyPEM []byte,
) (*etree.Document, error) {

	doc := etree.NewDocument()
	envEvento := doc.CreateElement("envEvento")
	envEvento.CreateAttr("versao", "1.00")
	envEvento.CreateAttr("xmlns", xmlnsNFe)

	idLoteEl := envEvento.CreateElement("idLote")
	idLoteEl.SetText(idLote)

	for _, ev := range eventos {
		// <evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">
		eventoEl := envEvento.CreateElement("evento")
		eventoEl.CreateAttr("versao", "1.00")
		// referência XML “correta” mostra xmlns aqui também
		eventoEl.CreateAttr("xmlns", xmlnsNFe)

		// <infEvento Id="ID...">
		infEventoEl := eventoEl.CreateElement("infEvento")
		idStr := buildIDEvento(ev.TpEvento, ev.ChNFe, ev.NSeqEvento)
		infEventoEl.CreateAttr("Id", idStr)

		infEventoEl.CreateElement("cOrgao").SetText(strconv.Itoa(ev.COrgao))
		infEventoEl.CreateElement("tpAmb").SetText(strconv.Itoa(ev.TpAmb))

		if ev.CNPJ != "" {
			infEventoEl.CreateElement("CNPJ").SetText(ev.CNPJ)
		} else if ev.CPF != "" {
			infEventoEl.CreateElement("CPF").SetText(ev.CPF)
		}

		infEventoEl.CreateElement("chNFe").SetText(ev.ChNFe)
		infEventoEl.CreateElement("dhEvento").SetText(ev.DhEvento.Format(time.RFC3339))
		infEventoEl.CreateElement("tpEvento").SetText(ev.TpEvento)
		infEventoEl.CreateElement("nSeqEvento").SetText(strconv.Itoa(ev.NSeqEvento))
		infEventoEl.CreateElement("verEvento").SetText(ev.VerEvento)

		detEventoEl := infEventoEl.CreateElement("detEvento")
		detEventoEl.CreateAttr("versao", "1.00")
		detEventoEl.CreateElement("descEvento").SetText(ev.DescEvento)

		// Assina ESTE infEvento e adiciona <Signature> como irmão (filho de <evento>)
		sigEl, err := buildSignatureForInfEvento(doc, infEventoEl, certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("erro assinando infEvento: %w", err)
		}
		eventoEl.AddChild(sigEl)
	}

	return doc, nil
}

// ID = "ID" + tpEvento + chNFe + nSeqEvento(2 dígitos)
func buildIDEvento(tpEvento, chNFe string, nSeq int) string {
	return fmt.Sprintf("ID%s%s%02d", tpEvento, chNFe, nSeq)
}

// ============================================================================
// Assinatura manual do infEvento (usa só o c14n do goxmldsig)
// ============================================================================

func buildSignatureForInfEvento(
	doc *etree.Document,
	infEventoEl *etree.Element,
	certPEM, keyPEM []byte,
) (*etree.Element, error) {
	idAttr := infEventoEl.SelectAttr("Id")
	if idAttr == nil || idAttr.Value == "" {
		return nil, fmt.Errorf("infEvento sem atributo Id")
	}
	refURI := "#" + idAttr.Value

	// 1) Canonicaliza o infEvento (C14N 2001)
	c14n := dsig.MakeC14N10RecCanonicalizer()
	canonInf, err := c14n.Canonicalize(infEventoEl)
	if err != nil {
		return nil, fmt.Errorf("erro canonicalizar infEvento: %w", err)
	}

	// 2) Digest SHA1 em base64
	h := sha1.Sum(canonInf)
	digestB64 := base64.StdEncoding.EncodeToString(h[:])

	// 3) Monta <Signature> (sem prefixo, com xmlns=xmlsig)
	sigEl := etree.NewElement("Signature")
	sigEl.CreateAttr("xmlns", dsig.Namespace) // "http://www.w3.org/2000/09/xmldsig#"

	signedInfoEl := sigEl.CreateElement("SignedInfo")

	cmEl := signedInfoEl.CreateElement("CanonicalizationMethod")
	cmEl.CreateAttr("Algorithm", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315")

	smEl := signedInfoEl.CreateElement("SignatureMethod")
	smEl.CreateAttr("Algorithm", "http://www.w3.org/2000/09/xmldsig#rsa-sha1")

	refEl := signedInfoEl.CreateElement("Reference")
	refEl.CreateAttr("URI", refURI)

	transformsEl := refEl.CreateElement("Transforms")

	tr1 := transformsEl.CreateElement("Transform")
	tr1.CreateAttr("Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")

	tr2 := transformsEl.CreateElement("Transform")
	tr2.CreateAttr("Algorithm", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315")

	dmEl := refEl.CreateElement("DigestMethod")
	dmEl.CreateAttr("Algorithm", "http://www.w3.org/2000/09/xmldsig#sha1")

	dvEl := refEl.CreateElement("DigestValue")
	dvEl.SetText(digestB64)

	// 4) Canonicaliza SignedInfo e assina com RSA-SHA1
	canonSignedInfo, err := c14n.Canonicalize(signedInfoEl)
	if err != nil {
		return nil, fmt.Errorf("erro canonicalizar SignedInfo: %w", err)
	}

	privKey, certDer, err := parsePrivateKeyAndCert(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("erro parse chave/cert: %w", err)
	}

	hashed := sha1.Sum(canonSignedInfo)
	sigBytes, err := rsa.SignPKCS1v15(nil, privKey, crypto.SHA1, hashed[:])
	if err != nil {
		return nil, fmt.Errorf("erro ao assinar SignedInfo: %w", err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	svEl := sigEl.CreateElement("SignatureValue")
	svEl.SetText(sigB64)

	// 5) KeyInfo + X509Data + X509Certificate
	kiEl := sigEl.CreateElement("KeyInfo")
	x509DataEl := kiEl.CreateElement("X509Data")
	x509CertEl := x509DataEl.CreateElement("X509Certificate")
	x509CertEl.SetText(base64.StdEncoding.EncodeToString(certDer))

	return sigEl, nil
}

// ============================================================================
// Util: carregar chave/cert PEM e extrair *rsa.PrivateKey + DER do certificado
// ============================================================================

func loadSigningCert(certPath, keyPath string) ([]byte, []byte, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("erro lendo cert PEM (%s): %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("erro lendo key PEM (%s): %w", keyPath, err)
	}

	// valida formato básico
	if _, _ = pem.Decode(certPEM); len(certPEM) == 0 {
		return nil, nil, fmt.Errorf("certificado PEM inválido em %s", certPath)
	}
	if _, _ = pem.Decode(keyPEM); len(keyPEM) == 0 {
		return nil, nil, fmt.Errorf("chave PEM inválida em %s", keyPath)
	}

	// opcional: testar como X509
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("erro decode cert PEM")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, nil, fmt.Errorf("erro parse certificado X509: %w", err)
	}

	return certPEM, keyPEM, nil
}

// extrai *rsa.PrivateKey e DER do cert pra X509Certificate
func parsePrivateKeyAndCert(certPEM, keyPEM []byte) (*rsa.PrivateKey, []byte, error) {
	// certificado
	cb, _ := pem.Decode(certPEM)
	if cb == nil {
		return nil, nil, fmt.Errorf("erro decode cert PEM")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("erro parse certificado: %w", err)
	}

	// chave privada (tenta PKCS8 e PKCS1)
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, nil, fmt.Errorf("erro decode chave PEM")
	}

	var pk any
	pk, err = x509.ParsePKCS8PrivateKey(kb.Bytes)
	if err != nil {
		pk2, err2 := x509.ParsePKCS1PrivateKey(kb.Bytes)
		if err2 != nil {
			return nil, nil, fmt.Errorf("erro parse private key (PKCS8/PKCS1): %v / %v", err, err2)
		}
		pk = pk2
	}

	rsaKey, ok := pk.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("chave privada não é RSA")
	}

	return rsaKey, cert.Raw, nil
}

// ============================================================================
// Parse da resposta SOAP em estrutura semântica
// ============================================================================

type retEnvEventoXML struct {
	IDLote   string          `xml:"idLote"`
	TpAmb    string          `xml:"tpAmb"`
	VerAplic string          `xml:"verAplic"`
	COrgao   string          `xml:"cOrgao"`
	CStat    string          `xml:"cStat"`
	XMotivo  string          `xml:"xMotivo"`
	RetEv    []retEventoXML  `xml:"retEvento"`
}

type retEventoXML struct {
	Versao    string          `xml:"versao,attr"`
	InfEvento infEventoRetXML `xml:"infEvento"`
}

type infEventoRetXML struct {
	TpAmb       string `xml:"tpAmb"`
	VerAplic    string `xml:"verAplic"`
	COrgao      string `xml:"cOrgao"`
	CStat       string `xml:"cStat"`
	XMotivo     string `xml:"xMotivo"`
	ChNFe       string `xml:"chNFe"`
	TpEvento    string `xml:"tpEvento"`
	XEvento     string `xml:"xEvento"`
	NSeqEvento  string `xml:"nSeqEvento"`
	DhRegEvento string `xml:"dhRegEvento"`
}

func parseManifestacaoResp(body []byte) (*ManifestacaoEventoResponse, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("retEnvEvento não encontrado na resposta")
			}
			return nil, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		if start.Name.Local == "retEnvEvento" {
			var raw retEnvEventoXML
			if err := dec.DecodeElement(&raw, &start); err != nil {
				return nil, err
			}
			return convertRetEnvToSemantic(&raw)
		}
	}
}

func convertRetEnvToSemantic(raw *retEnvEventoXML) (*ManifestacaoEventoResponse, error) {
	resp := &ManifestacaoEventoResponse{
		LoteID:           raw.IDLote,
		Motivo:           raw.XMotivo,
		VersaoAplicativo: raw.VerAplic,
	}

	resp.Ambiente, _ = strconv.Atoi(raw.TpAmb)
	resp.CodigoOrgao, _ = strconv.Atoi(raw.COrgao)
	resp.CodigoStatus, _ = strconv.Atoi(raw.CStat)

	for _, ev := range raw.RetEv {
		inf := ev.InfEvento
		item := ManifestacaoEventoItem{
			Motivo:          inf.XMotivo,
			ChNFe:           inf.ChNFe,
			TipoEvento:      inf.TpEvento,
			DescricaoEvento: inf.XEvento,
			VersaoAplicativo: inf.VerAplic,
		}

		item.Ambiente, _ = strconv.Atoi(inf.TpAmb)
		item.CodigoOrgao, _ = strconv.Atoi(inf.COrgao)
		item.CodigoStatus, _ = strconv.Atoi(inf.CStat)
		item.NumeroSequencial, _ = strconv.Atoi(inf.NSeqEvento)

		if inf.DhRegEvento != "" {
			if t, err := time.Parse(time.RFC3339, inf.DhRegEvento); err == nil {
				item.DataRegistro = t
			}
		}

		resp.Eventos = append(resp.Eventos, item)
	}

	return resp, nil
}
