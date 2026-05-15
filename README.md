# oom-signal

Демон, мониторящий потребление памяти контейнера через cgroup и отправляющий сигнал процессу при превышении порога.

Приложение запускает oom-signal как фоновый процесс. При превышении порога oom-signal отправляет настроенный сигнал (по умолчанию SIGUSR1) в указанный PID (по умолчанию 1). Обработка сигнала — зона ответственности приложения.

## Сборка

```bash
make build
```

Бинарник: `bin/oom-signal`

## Использование

```
oom-signal [--threshold=0.8] [--poll-interval=100ms] [--pid=1] [--signal=SIGUSR1]
```

| Параметр | Тип | По умолчанию | Описание |
|----------|-----|--------------|----------|
| `--threshold` | fraction или MB | 0.8 | Порог: доля 0.0–1.0 (напр. `0.85`) или запас в МБ (напр. `100MB`) |
| `--poll-interval` | duration | 100ms | Интервал опроса cgroup |
| `--pid` | int | 1 | PID процесса, в который отправить сигнал при превышении порога |
| `--signal` | string | SIGUSR1 | Сигнал для отправки (имя, напр. SIGTERM, или номер) |

## Пример запуска

С управлением жизненным циклом:

```python
import subprocess, os

monitor = subprocess.Popen(["oom-signal", "--threshold=100MB", "--pid=" + str(os.getpid())])

# Приложение работает...
# При получении SIGUSR1 — обработать ситуацию

monitor.terminate()
```

```go
cmd := exec.Command("oom-signal", "--threshold=100MB", "--pid="+strconv.Itoa(os.Getpid()))
cmd.Start()

// Приложение работает...
// При получении SIGUSR1 — обработать ситуацию

cmd.Process.Signal(syscall.SIGTERM)
cmd.Wait()
```

Без управления — oom-signal работает до остановки контейнера:

```python
import subprocess, os

subprocess.Popen(["oom-signal", "--pid=" + str(os.getpid())])

# Приложение работает...
# При завершении основного процесса контейнер остановится, oom-signal будет убит
```

```go
exec.Command("oom-signal", "--pid="+strconv.Itoa(os.Getpid())).Start()

// Приложение работает...
// При завершении основного процесса контейнер остановится, oom-signal будет убит
```

С Gunicorn — SIGUSR1 в master-процесс вызывает graceful restart воркеров (освобождение памяти):

```python
# gunicorn.conf.py
import subprocess

def on_starting(server):
    subprocess.Popen(["oom-signal"])
```

## Как работает

1. При запуске определяет версию cgroup (v1 или v2) и пути к файлам памяти один раз.
2. Периодически читает `usage` / `limit` из cgroup.
3. При превышении порога отправляет сигнал в указанный PID. Сигнал отправляется один раз на каждое пересечение порога снизу вверх (rising edge) — если память освободилась и снова превысила порог, сигнал отправится повторно.
4. Порог может быть задан:
   - Долей: `--threshold=0.8` — срабатывает при `usage/limit > 0.8`
   - Запасом в МБ: `--threshold=100MB` — срабатывает, когда свободной памяти < 100 МБ
5. Останавливается при получении SIGTERM или SIGINT.

## Поведение при безлимитном контейнере

Если cgroup limit = `max` (v2) или значение >= 2^62 (v1), oom-signal завершается с exit-кодом 1 и сообщением в stderr — мониторить нечего.

## Структура проекта

| Файл | Назначение |
|------|-----------|
| `src/cmd/main.go` | Точка входа, парсинг флагов, запуск монитора, обработка сигналов остановки |
| `src/internal/memory_reader/memory_reader.go` | Интерфейс MemoryReader, фабрика NewMemoryReader, определение версии |
| `src/internal/memory_reader/cgroup_v1.go` | Реализация MemoryReader для cgroup v1 |
| `src/internal/memory_reader/cgroup_v2.go` | Реализация MemoryReader для cgroup v2 |
| `src/internal/signal/signal.go` | Парсинг сигнала: имя (SIGUSR1) или номер |
| `src/internal/threshold/threshold.go` | Тип Threshold: парсинг и проверка порога (процент / МБ) |
| `src/internal/monitor/monitor.go` | Мониторинг памяти с тикером, отправка сигнала по rising edge |

## Логирование

Все события выводятся в stderr с префиксом `[oom-signal]`:

```
[oom-signal] starting: threshold=0.80 poll-interval=100ms pid=1 signal=SIGUSR1
[oom-signal] detected cgroup v2
[oom-signal] memory threshold exceeded: usage 838860800 / limit 1073741824 (free 234881024 bytes, threshold 80%)
[oom-signal] signal sent to pid 1
```

## Тестирование

Юнит-тесты:

```bash
make test
```

Функциональное тестирование в Docker — проверка доставки сигнала приложению при превышении порога памяти:

```bash
make test-threads     # потоки (ThreadPoolExecutor)
make test-processes   # процессы (ProcessPoolExecutor)
```

Подробнее — в [test/README.md](test/README.md).

## Зависимости

Только стандартная библиотека Go. Внешних зависимостей нет.
