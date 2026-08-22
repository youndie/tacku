---
id: B-31
title: "Подпись кнопки завершения сценария — `submitLabel` у `wizard_screen`"
status: done
priority: P2
size: S
stage: s4-product
blocked_by: [B-26]
---

# B-31 — Подпись кнопки завершения сценария — `submitLabel` у `wizard_screen`

Хром многошагового сценария рисует клиент — прогресс и переходы он собирает из `stepIndex`,
`totalSteps`, `canGoBack` и `formId` (§11.1, §11.4). Значит и подпись кнопки завершения живёт в
клиенте, то есть **общая для всех сценариев**: «Finish».

`KompotComponentWizardScreen` несёт `formId`, `stepId`, `stepIndex`, `totalSteps`, `canGoBack` и
`content` — ни одного поля подписи. Проверено по схеме.

**Вопрос «нужно ли поле вообще» снят раундом 3 дизайна: нужно, но узко.** Список нетерпимых
сценариев закрытый — удаление доски, приглашение в пространство, выдача агенту прав от вашего
имени, — и из него выведено правило: **своя подпись обязана быть, если шаг необратим, действует за
пределами продукта или является согласием.** Правило важнее списка: по нему четвёртый случай
опознаётся без нового раунда. Предел длины — 32 знака («Allow my agent to act for me» это 30, и
подвал под неё уже не резиновый); дальше сервер обрезает сам.

- **Для «New task» общая подпись терпима, для удаления или оплаты — нет.** «Finish» на шаге,
  который безвозвратно удаляет доску, — это ровно тот случай, когда интерфейс не сказал, что
  сейчас произойдёт. Автор макета назвал это сам, рисуя хром, и пока рисует общую.
- **Задача маленькая, но это изменение протокола.** Добавление поля со значением по умолчанию —
  совместимое изменение по §15, версию менять не нужно, но клиент всё равно обязан уметь его
  прочитать. Отсюда зависимость от [B-26](B-26-client-capability-flag.md): старый клиент поле
  проигнорирует и покажет «Finish», то есть деградация мягкая — но знать об этом надо заранее, а не
  обнаружить на сценарии удаления.
- Отвергнуто: рисовать свою кнопку завершения внутри `content`. Тогда на экране две кнопки — своя
  и клиентская, — и это ровно то расхождение, которое раунд 1 нашёл у шапки шага.
- Не покрывает: подпись кнопок «Back» и «Cancel». Они не несут смысла операции.

- AC: решение записано; если «да» — поле есть в профиле, сценарий удаления показывает свою подпись,
  а клиент без поддержки показывает «Finish» и не падает.
- Anchors: `client/spec/`, `docs/design/design-review.md`.

## Сделано

**Классификация: дыра спеки.** Не «поле есть, а мы им не пользуемся»: поля нет нигде — ни в файле
схемы, ни у опубликованного типа, ни в профиле сборки, и завести его у себя приложение не может.

### Что `wizard_screen` несёт на самом деле

Свойств девять, и ни одно из них не слово:

| Где смотрели | Что там |
|---|---|
| `spec/kompot-wizard.schema.json`, `KompotComponentWizardScreen` | `type`, `id`, `modifiers`, `formId`, `stepId`, `stepIndex`, `totalSteps`, `canGoBack`, `content` |
| опубликованный тип `WizardScreenComponent` (`javap` по `kompot-wizard-jvm` `0.17.0.24`, он же `SerialDescriptor` в тесте) | те же восемь без дискриминатора |

Файл схемы в `spec/` **побайтово совпадает** с `kompot-spec/schema/kompot-wizard.schema.json` из
опубликованного `kompot-spec-0.17.0.24.jar` — то есть отсутствие поля принадлежит контракту, а не
нашей генерации. Это стоило проверить: расхождение файла с артефактом было бы дефектом генератора,
и тогда задача была бы совсем другой.

Что делает молчание видимым — сравнение, а не сам список: **всякий орган управления, который на
экран ставит сервер, несёт свои слова** (`button.text`, `text_input.label`, `select_option.label`,
`read_only_field.helperText`, `screen_route.title`). Хром сценария — единственное, что клиент
ставит сам, и единственное место, где слов у сервера нет вовсе. Записано как
[Q-44](../research/questions.md).

### Почему это не чинится у себя

Два обхода выглядели доступными, и оба закрыты:

- **Расширить профиль.** Механизм, которым закрыта [Q-24](../research/questions.md), объявляет
  **тип** — `date_input` в `KompotComponent`, `date_field` в `FormFieldDefinition`. Добавить своё
  **свойство** чужому типу нечем; завести свой компонент вместо `wizard_screen` — значит потерять
  хром, ради которого сценарий и берут у тулкита. [Q-45](../research/questions.md).
- **Возить лишним ключом.** Схема лишний ключ допускает (`additionalProperties: true`), поэтому
  маршрут выглядит открытым — так уже сделано с `fieldId` в теле ошибки. **Измерено:** тело
  `wizard_screen` с лишним `submitLabel` парсер тулкита (`kompotJson` `0.17.0.24`) разбирает без
  ошибки в `WizardScreenComponent`, и обратная сериализация ключа уже не содержит. Расширение
  объектом работает там, где читающая сторона — свой код; хром рисует сам тулкит, и до него ключ не
  доходит. [Q-46](../research/questions.md).

### Что появилось в коде

Утверждение «поля нет» перестало быть абзацем. Проверок три, и они смотрят на разные артефакты
нарочно: файл схемы описывает, что **может ехать**, опубликованный тип — что **может быть удержано**
после того, как приехало.

- [`server/internal/spec/wizard_test.go`](../../server/internal/spec/wizard_test.go) —
  `TestTheChromeOfAScenarioCarriesNoWordsFromTheServer` читает свойства `wizard_screen` через
  профиль и сверяет их со словарём подписей; `TestTheSameDetectorFindsTheWordsOnAControlTheServerPlaces`
  — тот же словарь на `button`/`text`/`text_input`/`select_input`, без него первая проверка была бы
  утверждением о детекторе, а не о хроме; `TestAnExtraKeyOnAWizardScreenIsValid` фиксирует
  `additionalProperties: true` — половину, из-за которой обход выглядит открытым.
- [`client/app/src/test/kotlin/tacku/app/WizardChromeTest.kt`](../../client/app/src/test/kotlin/tacku/app/WizardChromeTest.kt)
  — элементы опубликованного типа и поведение лишнего ключа у парсера тулкита.

Обе проверки формы краснеют в тот день, когда поле появится: закрытую наверху дыру иначе замечают
перечитыванием схемы руками, то есть никогда. Модуль `kompot-wizard` подключён к `:app` как
`testImplementation` — продукту он не нужен, сценарных экранов клиент не рисует.

### Мутации

Каждая — правка файла, прогон, возврат файла из копии.

| Что сломано | Что покраснело |
|---|---|
| `submitLabel` добавлен в свойства `wizard_screen` в `spec/kompot-wizard.schema.json` | `TestTheChromeOfAScenarioCarriesNoWordsFromTheServer` — обе половины: и список свойств, и детектор подписей |
| `additionalProperties` у `KompotComponentWizardScreen` заменено на `false` | `TestAnExtraKeyOnAWizardScreenIsValid` |
| словарь подписей заменён на слово, которого в контракте нет | `TestTheSameDetectorFindsTheWordsOnAControlTheServerPlaces` |
| в ожидаемом списке элементов `content` заменено на `submitLabel` | `the published wizard screen has no field for the label of its finish button` |
| проверка обратной сериализации ищет `formId` вместо `submitLabel` | `an extra label on a wizard screen survives decoding and arrives nowhere` — без неё тело могло быть пустым, и проверка проходила бы вхолостую |

### Текст наверх (задача в kompot не заводилась)

> **`wizard_screen` has no way to name its own finish button.**
>
> A wizard screen carries `formId`, `stepId`, `stepIndex`, `totalSteps`, `canGoBack` and `content`.
> The chrome — Next, Back, Finish — is drawn by the client, so the finish button reads the same in
> every scenario of a build. Every other control the server places on a screen carries its own
> words: `button.text`, `text_input.label`, `select_option.label`, `read_only_field.helperText`,
> `screen_route.title`. The chrome is the only thing the client places on its own, and the only
> place where the server has no words at all.
>
> That is tolerable while the last step of a flow creates something, and not tolerable when it is
> irreversible, acts outside the product, or is a consent: "Finish" under a step that deletes a
> board looks harmless, which is exactly the cost.
>
> Neither route around it works from an application:
> * the extension mechanism added in 0.17 declares a **wire type**, not a property, so a gap inside
>   a toolkit component cannot be closed by a deployment on any stack. Replacing `wizard_screen`
>   with an application component loses the chrome it is wanted for;
> * an extra key validates (`additionalProperties: true`) and arrives nowhere: measured against
>   `kompotJson` 0.17.0.24, a `wizard_screen` body with an extra `submitLabel` decodes into
>   `WizardScreenComponent` without error and re-encodes without the key. Object extension works
>   where the reading side is the application's own code; the chrome is drawn by the toolkit.
>
> Suggested: an optional label field on `wizard_screen`, with the client's current wording as the
> default. A client released before it keeps showing "Finish", so the degradation is soft — which
> is precisely why it is worth naming now rather than discovering it under a delete flow.

### Что НЕ сделано и почему

- **Подпись не проверена на живом экране, и проверить её нечем.** Сценарных эндпоинтов у сервера
  нет — это [B-39](B-39-wizard-endpoints.md), она идёт параллельно. Значит вся эта задача закрыта
  **по контракту, а не по поведению**: доказано, что назвать кнопку нечем, и не показано, как
  выглядит экран, на котором она названа. Граница названа прямо, чтобы её не приняли за проверенное.
- **Поле в профиле не появилось** — его туда нельзя положить, см. Q-45. Вторая половина AC
  («сценарий удаления показывает свою подпись») недостижима до ответа наверху, а не отложена.
- **Предел в 32 знака не реализован.** Обрезать нечего: значения нет. Правило остаётся в описании
  задачи и вступит в силу вместе с полем.
- **Зависимость от [B-26](B-26-client-capability-flag.md) снята не так, как ожидалось.** Флага
  возможностей клиента в протоколе нет вовсе — это ответ, который B-26 записала. Поэтому «вводить
  поле вместе с флагом» неисполнимо: порядок выкатки (§15) и мягкая деградация — всё, чем это
  обеспечивается, и первое, что придётся сказать в обращении наверх.
- **Задача в kompot не заведена** — по правилам этого прогона текст лежит выше, а заводится
  отдельно.
