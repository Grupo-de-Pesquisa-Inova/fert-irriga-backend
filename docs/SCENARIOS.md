# Cenários hidráulicos

O catálogo `internal/scenario/catalog.json` é extraído de `Blender/estufa.py` pelo
exportador `Blender/export_web.py`. São 11 combinações com uma única fonte,
bomba M e destino L1–L3, L4–L6, L1–L6 ou retorno de limpeza. Não existe conversão
segura implícita entre estas válvulas e as seis saídas legadas do projeto.

## Operação

- A aba **Cenários** oferece prévia do modelo original, órbita, zoom, deslocamento,
  vistas predefinidas e inspeção das válvulas. A prévia não envia comandos.
- A visualização ao vivo usa `status_sistema.operacao.saidas_ativas`, recebido
  pelo WebSocket. Cores são estado de saídas informado pelo controlador; somente
  o sensor de fluxo confirma passagem de água.
- Execução imediata e horários únicos/diários usam o mesmo serviço. Os horários
  são armazenados em UTC. Repetição diária significa 24 horas a partir do horário
  escolhido (a UI informa o fuso do navegador).
- O servidor é a autoridade de agendamento dos cenários. Ele precisa estar ligado
  e conectado no horário. O timer de **desligamento** pertence ao controlador e
  deve continuar independentemente do servidor, navegador e MQTT.
- Início atrasado em mais de 60 segundos gera `schedule_missed`, sem irrigação
  tardia. Na recuperação de uma recorrência, o registro informa quantos horários
  venceram durante a indisponibilidade.
- `pending` não significa ligado; `running` exige confirmação do dispositivo;
  `completed` exige confirmação de término. Falhas de envio/ACK podem deixar a
  execução `uncertain`, bloqueando novas ativações até confirmação de parada.
- Alarmes aparecem como avisos dentro do painel, inclusive fora da aba Cenários.
  Não há envio de WhatsApp, SMS, e-mail ou push. Se o próprio servidor está fora
  do ar, o registro e o aviso de horário perdido aparecem quando ele retorna.
- Agendamentos antigos continuam pertencendo ao RTC legado. O worker que
  inventava auditorias de sucesso sem acionar o dispositivo foi desativado.

## Contrato para o firmware real

Só anunciar `capabilities: ["scenarios_v1"]` na telemetria após implementar os
intertravamentos e timer abaixo. A versão física está no repositório Fertirriga-Esp32
(relés de cada saída em `HARDWARE.md`); o código incluído aqui é um simulador sem GPIO. Os comandos legados ficam bloqueados
para dispositivos que anunciam esta capacidade.

Receber em `fertirriga/{device_id}/cenario`, QoS 1, **sem retain**:

```json
{
  "protocol": "scenarios_v1",
  "action": "start",
  "run_id": "UUID",
  "scenario_id": "cenario-09",
  "outputs": ["VA", "VG", "VL13", "M"],
  "duration_sec": 300,
  "expires_at": "2026-10-10T12:00:15Z"
}
```

Requisitos do controlador:

1. Validar ID do cenário, conjunto exato de saídas, duração 1–7200 s e expiração.
   Rejeitar relógio dessincronizado, emergência, manutenção e falha hidráulica.
2. Uma execução por vez, inclusive em relação a comandos e agendas locais.
   Aplicar sequência física segura: bomba desligada, selecionar fonte/destino,
   confirmar válvulas e só então ligar bomba. Intertravamentos físicos e tempos
   dependem do hardware e precisam ser calibrados no projeto de firmware.
3. Deduplicar `run_id` de forma persistente: retransmissão não reinicia o timer.
   Usar relógio monotônico para desligar, mesmo sem internet. Ao reiniciar,
   deixar as saídas seguras e reportar a execução interrompida.
4. Parar a bomba antes de fechar as válvulas. Falha de fluxo, emergência e tempo
   máximo devem interromper localmente; informar o código e causa detectada.
5. Aceitar `{"protocol":"scenarios_v1","action":"stop","run_id":"UUID"}`
   de forma idempotente. Confirmar somente após desligamento seguro. Um stop
   recebido antes do start deve impedir que esse start atrasado abra as saídas.
6. Publicar estados em `fertirriga/{device_id}/ack` (ou `/evento`), com QoS 1;
   persistir eventos de término offline e reenviá-los após reconectar. O servidor
   aceita repetição sem regredir estados finais e valida dispositivo + execução.

```json
{"run_id":"UUID","status":"running","started_at":"2026-10-10T12:00:00Z"}
{"run_id":"UUID","status":"completed","finished_at":"2026-10-10T12:05:00Z"}
{"run_id":"UUID","status":"failed","error_code":"flow_fault","message":"Sem fluxo; bomba desligada.","finished_at":"2026-10-10T12:00:10Z"}
{"run_id":"UUID","status":"cancelled","finished_at":"2026-10-10T12:00:12Z"}
```

O sensor pode confirmar fluxo, não volume aplicado ou sucesso agronômico. Uma
queda de comunicação é identificada como falta de confirmação; não é rotulada
automaticamente como “internet caiu” ou “não irrigou” sem evidência do dispositivo.

## API

| Método | Rota | Uso |
|---|---|---|
| GET | `/api/v1/scenarios` | Catálogo canônico |
| GET / POST | `/api/v1/scenario-runs` | Histórico filtrado por `device_id` / ativar |
| POST | `/api/v1/scenario-runs/{id}/stop?device_id=...` | Solicitar parada |
| GET / POST | `/api/v1/scenario-plans` | Listar por `device_id` / criar |
| PUT / DELETE | `/api/v1/scenario-plans/{id}` | Editar / excluir logicamente |

DELETE também recebe `device_id` na query. PUT recebe o objeto completo, incluindo
`enabled`; pausa não interrompe execução em andamento. O histórico é preservado.
A migração `015_scenarios.sql` é aplicada automaticamente ao iniciar o backend.

## Validação local

```powershell
# Requer broker e banco de teste locais; nunca usar GPIO real para os testes.
go run ./cmd/scenario-simulator -broker mqtt://127.0.0.1:51883 -device esp32-001
# Falhas reproduzíveis:
go run ./cmd/scenario-simulator -broker mqtt://127.0.0.1:51883 -device test-noack -fault noack
go run ./cmd/scenario-simulator -broker mqtt://127.0.0.1:51883 -device test-flow -fault flow_fault
# Testes de banco criam/removem apenas um schema próprio em PostgreSQL local.
$env:SCENARIO_TEST_DATABASE_URL = 'postgres://USER:PASSWORD@127.0.0.1:55432/fertirriga?sslmode=disable'
go test ./...
go vet ./...
```

O simulador usa estado em memória e não substitui a persistência, o watchdog e
os testes físicos exigidos no firmware. Seu timer é independente do publicador MQTT.

Para atualizar o modelo e os dois catálogos gerados, executar da raiz:

```powershell
& 'C:/Program Files/Blender Foundation/Blender 5.2/blender.exe' -b Blender/estufa.blend -P Blender/export_web.py
```

O `.blend` original é preservado. A exportação reduz os materiais procedurais a
cores para renderização em tempo real. O GLB tem aproximadamente 4,8 MB e só é
carregado ao abrir Cenários; a biblioteca Three.js também é carregada sob demanda.
