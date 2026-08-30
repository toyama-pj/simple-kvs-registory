# Simple Chirp

Semtech UDP Packet Forwarder から LoRaWAN uplink を受信し、Cayenne LPP の計測値を PostgreSQL に保存して Fiber Web API で提供する Go アプリケーションです。DB アクセスとマイグレーションには GORM を使用します。

## データモデル

```text
User ── OrganizationMembership ── Organization
                                      └── Namespace
                                            ├── NamespaceAccessPermission ── User
                                            └── Device ── Measurement

Semtech UDP datagram ── SemtechUDPLog
```

- Organization は複数の Namespace を持ちます。
- Namespace は複数の LoRaWAN Device を持ちます。
- User の Organization 所属と、Namespace の read/write/admin 権限は別々に管理します。
- Measurement は Device と Namespace の両方に紐付きます。
- すべての Semtech UDP datagram は `semtech_udp_log` に監査記録されます。

## 起動

`.env.sample` をコピーし、少なくともセッションキー暗号化用マスターキーを生成し直してください。

```bash
cp .env.sample .env
openssl rand -hex 32
# 出力を .env の DEVICE_SESSION_KEY_ENCRYPTION_KEY に設定
docker compose up --build
```

公開ポートは Web API の TCP `3000` と Semtech UDP の UDP `1700` です。Compose は PostgreSQL 16 を起動し、アプリの接続先を自動的に `db:5432` へ上書きします。

ローカルで直接起動する場合は `.env` の `DATABASE_DSN` をローカル PostgreSQL に合わせ、次を実行します。

```bash
make run
```

## Semtech UDP と LoRaWAN

標準 Packet Forwarder の `rxpk.data` は Cayenne LPP の平文ではなく、暗号化済み LoRaWAN PHYPayload です。そのため Device 登録には次の値が必要です。

- DevEUI: 16 桁の16進数
- DevAddr: 8 桁の16進数
- AppSKey / NwkSKey: 各32桁の16進数

現在の組み込み受信処理は LoRaWAN 1.0.x の data uplink（ABP または確立済みセッション）を対象とします。MIC、uplink frame counter、FPort を検査し、AppSKey で復号した後に Cayenne LPP を解析します。OTAA Join、LoRaWAN 1.1、downlink のスケジューリングが必要な構成では、このアプリの前段に LoRaWAN Network Server を置いてください。

対応する Cayenne LPP 型は digital input/output、analog input/output、illuminance、presence、temperature、relative humidity、accelerometer、barometric pressure、gyrometer、GPS です。

## 主な API

すべて `/api/v1` 配下です。管理・閲覧 API は User Bearer Token を必要とします。

| Method | Path | 用途 |
| --- | --- | --- |
| `POST` | `/organizations` | Organization 作成 |
| `GET` | `/organizations` | 所属 Organization 一覧 |
| `POST` | `/organizations/{id}/namespaces` | Namespace 作成 |
| `GET` | `/organizations/{id}/namespaces` | Namespace 一覧 |
| `POST` | `/namespaces/{id}/devices` | Device とセッションキー登録 |
| `GET` | `/namespaces/{id}/devices` | Device 一覧 |
| `GET` | `/namespaces/{id}/measurements` | Namespace の計測値取得 |
| `GET` | `/devices/{id}` | Device 取得 |
| `PATCH` | `/devices/{id}` | Device 名・有効状態・セッションキー更新 |
| `DELETE` | `/devices/{id}` | Device 無効化・削除 |
| `GET` | `/devices/{id}/measurements` | Device の計測値取得 |

Measurement 取得では `before`、`after`（Unix 秒）、`device_id`、`name`、`channel`、`limit`（1〜500）を利用できます。既存の `/data` と `/cfg` API も後方互換用に維持しています。

Device 登録例:

```json
{
  "name": "greenhouse-1",
  "dev_eui": "0102030405060708",
  "dev_addr": "26011BDA",
  "app_s_key": "00112233445566778899AABBCCDDEEFF",
  "nwk_s_key": "FFEEDDCCBBAA99887766554433221100"
}
```

AppSKey と NwkSKey は API レスポンスおよびアクセスログでは伏せられ、PostgreSQL には AES-256-GCM で暗号化して保存されます。
セッションキーは必ず2つ同時に更新し、更新時には uplink frame counter が新しいセッション用にリセットされます。

詳細な認証・権限・監査仕様は [wiki/api-contract.md](wiki/api-contract.md) を参照してください。
