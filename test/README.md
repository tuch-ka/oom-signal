# Функциональное тестирование в Docker

Тест проверяет, что oom-signal корректно обнаруживает превышение порога памяти и доставляет сигнал приложению.

## Что тестируется

1. oom-signal запускается в контейнере с cgroup v2 и лимитом памяти
2. Приложение (Python-скрипт) постепенно выделяет память, пока не будет превышен порог 80%
3. oom-signal отправляет SIGUSR1 в PID приложения
4. Приложение получает сигнал и корректно завершается

## Запуск

```bash
make test-threads       # потоки (ThreadPoolExecutor)
make test-processes     # процессы (ProcessPoolExecutor)
make test-burst         # тест динамического интервала
```

### Параметры

| Переменная | По умолчанию | Описание |
|-----------|--------------|----------|
| `MEMORY_LIMIT` | `512m` | Лимит памяти контейнера (`docker --memory`) |
| `MEMORY_STEP` | `64` | Шаг выделения памяти в МиБ каждым воркером за итерацию |
| `WORKERS` | `4` | Количество воркеров (потоков или процессов) |

Пример с кастомными параметрами:

```bash
MEMORY_LIMIT=256m MEMORY_STEP=32 WORKERS=2 make test-threads
```

## Режимы

### threads

Воркеры (по умолчанию 4) выделяют память в общем адресном пространстве. RSS процесса-родителя отражает суммарное потребление. При достижении 80% от лимита приходит SIGUSR1.

### processes

Воркеры (по умолчанию 4) выделяют память, каждый со своим RSS. Cgroup usage — сумма всех процессов в контейнере. При достижении 80% от лимита приходит SIGUSR1 в PID 1 (основной процесс).

### burst (динамический интервал)

Тест проверяет, что динамический интервал опроса срабатывает при резком росте потребления памяти:

1. Аллоцирует память до ~60% лимита (стабильный уровень, монитор на дефолтном интервале)
2. Делает резкую burst-аллокацию, пересекая порог 80%
3. Замеряет задержку от начала burst до получения SIGUSR1 (должна быть < 200ms)
4. Проверяет в stderr лог смены интервала (`poll interval: ... → ...`)

## Ожидаемый результат

В логах контейнера должна быть следующая цепочка событий:

```
[oom-signal] starting: threshold=0.80 pid=1 signal=SIGUSR1 reader=cgroup v2
[oom-signal] poll interval: 100ms → 1ms
[oom-signal] memory threshold exceeded: usage 434974720 / limit 536870912 (free 101896192 bytes, threshold 80%)
[oom-signal] signal sent to pid 1
Received SIGUSR1 from oom-signal
```

Контейнер завершается с exit 0.

Без oom-signal при том же сценарии контейнер был бы убит OOM Killer (exit 137).

## Файлы

| Файл | Назначение |
|------|-----------|
| `eat_memory.py` | Python-скрипт: запуск oom-signal, обработка SIGUSR1, постепенное выделение памяти |
| `burst_test.py` | Python-скрипт: тест динамического интервала (burst-аллокация, замер задержки) |
| `Dockerfile` | Multi-stage build: сборка Go-бинарника + Python-окружение |

## Сборка вручную

```bash
docker build -t oom-signal-test -f test/Dockerfile .
docker run --rm --memory=512m oom-signal-test threads --step 64 --workers 4
docker run --rm --memory=512m oom-signal-test processes --step 64 --workers 4
docker run --rm --memory=512m --entrypoint python oom-signal-test /opt/oom-signal/burst_test.py
```
