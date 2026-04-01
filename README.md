# palisade
Palisade
started at: 15.08.2025

## Usage

### go run ./cmd/cli/main.go get_coin_list
Get all available coin from Mexc and update db

## Telegram: уведомления (компактный формат)

Сообщения из команд **`process`** (`palisade_process`) и **`process-sell`** (`palisade_process_sell`) — **одна строка**, поля разделены символом **`·`**. В HTML-режиме Telegram заголовок в `<b>…</b>`, id ордеров в `<code>…</code>`.

### Типы сообщений

| Заголовок | Когда |
|-----------|--------|
| **📥 Покупка** | Размещён лимитный ордер на покупку (process) |
| **⚠️ Нет на бирже** | Открытый buy не найден на бирже → в БД проставлен `cancel_date` |
| **💰 Продажа завершена** | Лимитный sell исполнен, сделка закрыта, посчитан P/L |
| **🚨 Маркет-продажа** | Вместо лимита выставлен маркет-sell (выход из диапазона / тайминг и т.д.) |
| **⏱️ Покупка отменена по времени** | Buy в статусе NEW дольше 2 ч → отмена на бирже и в БД |
| **❌ Покупка CANCELED / REJECTED / EXPIRED** | Статус покупки с биржи |
| **✅ Покупка FILLED** | Покупка полностью исполнена (перед выставлением sell) |
| **💸 Лимит на продажу** | После FILLED выставлен лимит на продажу по resistance |
| **⚠️ Частичная отмена покупки** | Статус `PARTIALLY_CANCELED` |
| **⚠️ Неизвестный статус покупки** | Статус buy не из ожидаемого набора |

### Расшифровка полей

| Маркер / фрагмент | Смысл |
|-------------------|--------|
| `S` / `R` | Support и resistance (нижняя / верхняя граница палисады) |
| `ордер` / `buy` / `sell` + `<code>…</code>` | ID ордера на бирже |
| `цена`, `px` | Цена лимита или срабатывания |
| `кол-во`, `qty` | Количество базового актива |
| `~… USDT` | Оценочная сумма в USDT |
| `баланс` / `своб` / `блок` | USDT на счёте: всего, свободно, в ордерах |
| `покупка A×B` | Цена покупки × объём |
| `баланс на откр` | USDT баланс на момент открытия сделки (для расчёта P/L) |
| `открыт` / дата в скобках | Время открытия buy в логе |
| `P/L` | Прибыль/убыток в USDT и % от баланса на открытии |
| `продажа … = … USDT` | Средняя цена продажи × исполненное кол-во = выручка |
| `исполнено X / Y` | Исполнено / изначально по частичной отмене |
| `БД cancel` | В логе сделки обновлена отмена / закрытие |
| `>2ч в NEW` | Правило таймаута для висящего лимита на покупку |
| хвост после причин | Для маркет-продажи — сжатое описание триггера (цена вверх/вниз/2 ч) |

Сообщения из **`process-multi`** этому формату не следуют (отдельный сводный отчёт).

## Ручной поток (trade_log_manual)

Отдельная таблица **`trade_log_manual`** — сделки не смешиваются с авто-`trade_log`. На уже развёрнутой БД выполните [sqlc/migrations/001_trade_log_manual.sql](sqlc/migrations/001_trade_log_manual.sql).

| Команда | Назначение |
|---------|------------|
| `process-manual` | Лимитная покупка по YAML → запись в `trade_log_manual` |
| `process-sell-manual` | Та же логика, что `process-sell`, но только по `trade_log_manual` |

```bash
go run ./cmd/cli/main.go process-manual --config process_manual.yaml
go run ./cmd/cli/main.go process-sell-manual
# при нескольких открытых manual-сделках:
go run ./cmd/cli/main.go process-sell-manual --config process_sell_manual.yaml
```

Примеры YAML: [config/examples/process_manual.yaml](config/examples/process_manual.yaml), [config/examples/process_sell_manual.yaml](config/examples/process_sell_manual.yaml).
