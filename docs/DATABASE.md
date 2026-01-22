# Dependências de Banco de Dados (Oracle)

Para o funcionamento correto do ZENITH-GO, as seguintes estruturas devem existir no banco de dados do Sankhya.

## 1. Tabelas Customizadas (AD)

| Tabela | Descrição | Uso no Código |
|--------|-----------|---------------|
| `AD_APPPERM` | Permissões do usuário WMS. | Controla flags: `TRANSF`, `BAIXA`, `PICK`, `CORRE`. |
| `AD_DISPAUT` | Controle de dispositivos móveis. | Vincula `CODUSU` ao `DEVICETOKEN`. |
| `AD_CADEND` | Cadastro de Endereços (Estoque). | Leitura de saldo e locais. |
| `AD_BXAEND` | Cabeçalho de movimentação. | Armazena data e usuário da operação. |
| `AD_IBXEND` | Itens da movimentação. | Registra produto, origem, destino e quantidade. |
| `AD_HISTENDAPP` | Histórico de correções. | Auditoria de inventário/correção de estoque. |

## 2. Views Obrigatórias

### `V_WMS_ITEM_DETALHES`
Utilizada pelo endpoint `/get-item-details`.

```sql
CREATE OR REPLACE VIEW NICOPRD.V_WMS_ITEM_DETALHES AS
WITH DERIVACOES AS (
  SELECT CODPROD, CODVOL, MAX(DESCRDANFE) AS DERIVACAO
  FROM TGFVOA
  GROUP BY CODPROD, CODVOL
)
SELECT
  ENDE.CODARM, ENDE.SEQEND, ENDE.CODRUA, ENDE.CODPRD,
  ENDE.CODAPT, ENDE.CODPROD, PRO.DESCRPROD, PRO.MARCA,
  ENDE.DATVAL, ENDE.QTDPRO, ENDE.ENDPIC, ENDE.NUMDOC,
  TO_CHAR(ENDE.QTDPRO) || ' ' || ENDE.CODVOL AS QTD_COMPLETA,
  DER.DERIVACAO
FROM AD_CADEND ENDE
JOIN TGFPRO PRO ON PRO.CODPROD = ENDE.CODPROD
LEFT JOIN DERIVACOES DER ON DER.CODPROD = ENDE.CODPROD AND DER.CODVOL = ENDE.CODVOL;

```

## 3. Stored Procedures

O sistema chama a procedure `NIC_STP_BAIXA_END` via serviço `ActionButtonsSP.executeSTP` (ActionID 20) para efetivar as baixas e transferências no ERP.
