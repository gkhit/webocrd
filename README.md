Web OCR service
===============

Web cервис распознования документов

Установка необходимого ПО
-------------------------
    $ apt install ocrmypdf tesseract-ocr-rus poppler-utils img2pdf imagemagick

Сборка
------
    $ git clone https://github.com/gkhit/webocrd.git
    $ cd webocrd
    $ ./build.sh

Переменные окружения
--------------------
| Переменная | Описание | По умолчанию |
| - | - | - |
| WEBOCRD_HTTP_ADDR | Адрес HTTP сервера ||
| WEBOCRD_MAX_FILE_SIZE | Максимальный размер загружаемого файла | 5 Мб |
| WEBOCRD_MAX_REQ_SIZE | Максимальный размер запроса | 50 Мб |

Установка службы
----------------

#### Создайте файл /etc/systemd/system/webocrd.service

    [Unit]
    Description=Web OCR service
    After=network.target
    StartLimitIntervalSec=0

    [Service]
    Type=simple
    Restart=always
    RestartSec=3
    User=root
    Group=root
    PIDFile=/var/run/webocrd.pid
    WorkingDirectory=/opt/webocrd
    EnvironmentFile=/opt/webocrd/env.conf
    ExecStart=/opt/webocrd/webocrd -d
    TimeoutStopSec=5

    [Install]
    WantedBy=multi-user.target

#### Запустите службу

    $ systemctl start webocrd

#### Добавте автозапуск службы при загрузке

    $ systemctl enable webocrd

Пример env.conf
---------------
    WEBOCRD_HTTP_ADDR="localhost:9090"
