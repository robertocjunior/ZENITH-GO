## 📚 API Reference & Examples

Below are practical examples of how to consume the endpoints.

> **Note on Headers:**
>
>   * `Authorization`: Must contain `Bearer <YOUR_JWT_TOKEN>` (returned upon login).
>   * `Snkjsessionid`: Must contain the Sankhya session cookie (returned upon login), required only for the **Transactions** route.

### 🔐 Authentication

#### Login

Performs dual authentication (Mobile + System) and returns the necessary tokens.

  - **Endpoint:** `POST /apiv1/login`

<!-- end list -->

```json
{
  "username": "YOUR_USERNAME",
  "password": "YOUR_PASSWORD",
  "deviceToken": "" 
}
```

> *Tip: If `deviceToken` is sent empty, the system generates a new one and returns it in the response.*

#### Logout

  - **Endpoint:** `POST /apiv1/logout`
  - **Header:** `Authorization: Bearer <TOKEN>`

-----

### 📦 Products & Stock

#### Search Items

Searches for products by code, description, or address in the warehouse.

  - **Endpoint:** `POST /apiv1/search-items`
  - **Header:** `Authorization: Bearer <TOKEN>`

<!-- end list -->

```json
{
  "codArm": 1,
  "filtro": "RICE" 
}
```

> *The filter accepts text (description) or numbers (product/address code).*

#### Item Details (Batch/Expiry)

Gets detailed data of an item at a specific address.

  - **Endpoint:** `POST /apiv1/get-item-details`
  - **Header:** `Authorization: Bearer <TOKEN>`

<!-- end list -->

```json
{
  "codArm": 1,
  "sequencia": "12345"
}
```

#### Picking Locations

Searches for alternative picking locations for replenishment.

  - **Endpoint:** `POST /apiv1/get-picking-locations`
  - **Header:** `Authorization: Bearer <TOKEN>`

<!-- end list -->

```json
{
  "codarm": 1,
  "codprod": 107010020,
  "sequencia": 12345
}
```

> *Note: `sequencia` here is the current address (to be excluded from the suggestion list).*

#### Daily History

Returns all movements and corrections made by the user on the current date.

  - **Endpoint:** `POST /apiv1/get-history`
  - **Header:** `Authorization: Bearer <TOKEN>`

<!-- end list -->

```json
{
  "dtIni": "25/11/2025",
  "dtFim": "25/11/2025",
  "codUsu": 0
}
```

> *If `codUsu` is 0, it returns everyone's history (if permitted).*

-----

### ⚡ Transactions (Movements)

This is the unified endpoint for write operations.

  - **Endpoint:** `POST /apiv1/execute-transaction`
  - **Required Headers:**
      - `Authorization: Bearer <TOKEN>`
      - `Snkjsessionid: <LOGIN_JSESSIONID>`

#### 1\. Stock Write-off (Consumption)

Removes a product from stock.

```json
{
  "type": "baixa",
  "payload": {
    "origem": {
      "codarm": 1,
      "sequencia": 12345,
      "endpic": "N" 
    },
    "quantidade": 10
  }
}
```

#### 2\. Transfer

Moves a product from one address to another.

```json
{
  "type": "transferencia",
  "payload": {
    "origem": {
      "codarm": 1,
      "sequencia": 12345,
      "codprod": 107010020
    },
    "destino": {
      "armazemDestino": 1,
      "enderecoDestino": "200", 
      "quantidade": 5,
      "criarPick": false
    }
  }
}
```

> *`criarPick: true` attempts to mark the destination as Picking (requires permission).*

#### 3\. Picking (Replenishment)

Similar to transfer, but with specific picking validations at the destination.

```json
{
  "type": "picking",
  "payload": {
    "origem": {
      "codarm": 1,
      "sequencia": 12345
    },
    "destino": {
      "armazemDestino": 1,
      "enderecoDestino": "500",
      "quantidade": 20
    }
  }
}
```

#### 4\. Stock Correction (Inventory)

Adjusts the quantity of an item (generates a correction script in the ERP).

```json
{
  "type": "correcao",
  "payload": {
    "codarm": 1,
    "sequencia": 12345,
    "newQuantity": 150
  }
}
```