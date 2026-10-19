# Acionamento individual das válvulas do 3D

O clique no corpo ou rótulo de uma válvula abre uma confirmação. Após confirmar,
o frontend envia `POST /api/v1/commands` com `target_channel` (VA, VAD1, VAD2,
VG, VL13, VL46, VRA1 ou VRA2), `action` (`open_valve` / `close_valve`) e
`parameters.expected_state` (booleano observado antes da confirmação).

O servidor exige telemetria recente e a capability `valves_v1`, registra o comando
e publica em `fertirriga/{device_id}/comando`, QoS 1, sem retain:

```json
{"protocol":"valves_v1","action":"set_valve","command_id":"UUID","target":"VL13","state":true,"expected_state":false,"expires_at":"2026-10-10T15:00:15Z"}
```

O controlador deve rejeitar comandos vencidos, deduplicar `command_id`, comparar
o estado esperado com a saída atual e reaplicar os intertravamentos localmente:
sem cenário em execução, sem bomba ligada, sem mistura de origens ou retorno
incompatível. O comando altera somente a válvula; nunca liga a bomba.
Fechar manualmente exige a mesma confirmação. Emergência deve desligar as saídas.

Após aplicar o comando, publicar em `fertirriga/{device_id}/ack`:

```json
{"protocol":"valves_v1","command_id":"UUID","target":"VL13","state":true,"status":"executed"}
```

Falhas usam `status: "rejected"` ou `"failed"` e `message`. Publicar também a
lista atualizada `status_sistema.operacao.saidas_ativas` na telemetria. O 3D usa
essa telemetria, sem atualização otimista. A interface só confirma sucesso após
ACK e estado correspondente; falta de ACK por 20 segundos é registrada como
`expired` (sem confirmação, não prova de que a válvula permaneceu fechada).
ACKs tardios válidos podem resolver esse estado. Não há reenvio automático.

O simulador local implementa este contrato sem GPIO. O firmware em
`Esp32/src/main.cpp` ainda usa seis saídas legadas e não anuncia `valves_v1`.
É necessário confirmar a correspondência dos oito nomes com os relés físicos
antes de adaptar/gravar esse firmware. Não remapear os nomes por suposição.
