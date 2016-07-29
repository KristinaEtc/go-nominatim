#Сервис геопривязки 
Перевод геокоординат в местоположение через сервис Nominatim.

Программа состоит из 2 пакетов: `go-stomp-server` и `go-stomp-nominatim`.

1. [go-stomp-server_1.0-1_all.deb](https://github.com/KristinaEtc/stomp/raw/dev/stompd/deb-package/go-stomp-server_1.0-1_all.deb)

Независимый от базы Nomintim сервер, работающий по протоколу STOMP (https://stomp.github.io/).<br/ >
Фактически является брокером сообщений (см. документацию протокола stomp). В данном контексте - связующее звено между клиентом, который посылает запрос 
на получение местоположения, и модулем `go-stomp-nominatim`.

```
Usage of ./go-stomp-server:
  -addr string
        Listen address (default ":61613")
  -conf string
        configfile with logins and passwords (default near a binary file)
```
 
2. [go-stomp-nominatim_1.0-1_all.deb](https://github.com/KristinaEtc/go-nominatim/raw/master/deb-package/go-stomp-nominatim_1.0-1_all.deb)

Сервис, который подписывается на go-stomp-server, обрабатывает эти запросы и ищет нужные координаты в базе данных Nominatim. 
После перенаправляет результат обратно go-stomp-server'у. 
Соответственно, для его работы Nominatim должен быть уже установлен.

```
Usage of ./go-stomp-nominatim:
  -conf string
        config file for Nominatim DB (default near a binary file)
  -login string
        Login for authorization (default "client1")
  -pwd string
        Passcode for authorization (default "111")
  -qFormat string
        queue format (default "/queue/")
  -queue string
        Destination queue (default "/queue/nominatimRequest")
  -server string
        STOMP server endpoint (default "localhost:61614")
```

## Установка пакетов

```
# sudo dpkg -i go-stomp-server_1.0-1_all.deb
# sudo dpkg -i go-stomp-nominatim_1.0-1_all.deb
```

## Проверка состояния. Конфигурирование и тестирование

### Конфигурационные файлы

Посмотреть, какие файлы установлены пакетом (аналогично для go-stomp-nominatim):
```
# dpkg-query -L go-stomp-server
```
Для расположения файлов по возможности было использована структура 
[FHS](http://stackoverflow.com/questions/1024114/location-of-ini-config-files-in-linux-unix).

* Исполняемые файлы: /usr/bin

  * `/usr/bin/go-stomp-server`<br/ >
  * `/usr/bin/go-stomp-nominatim`

* Конфигурационные файлы: /usr/share/pachage-name

Для каждой программы, установленной из .deb-пакета, инсталлируется 2 конфигурационных файла с форматом JSON: один для логгера, второй для общей конфигурации
(на данный момент в общей конфигурации для модулей хранятся только логины и пароли для подключения). Бинарники из .deb-пакета ищут конфиги
в /usr/share со **_стандартным именем_**: свое имя с добавлением префикса ".config" для общей конфигурации и ".logconfig" для логов, например: /usr/share/go-stomp-server/go-stomp-server.logconfig

Общие конфиги (то есть путь - полный или относительный - вместе с именем) можно установить и через фраг `-conf=`. 
Если файл будет не найден/с плохим форматом/поврежденным, будет осуществлен поиск рядом с бинарным файлом со стандартным именем.

a) .logconfig<br/ >
Положение конфигурационного файла для логгера и его имя определяется при сборке пакета и пользователем не изменяется.
Если файл будет не найден/удален, будут использованы стандартные настройки, которые совпадают с содержимым конфига, установленного с пакетом,

```
#cat /usr/share/go-stomp-nominatim/go-stomp-nominatim.logconfig
{
	"Filenames": {
		"ERROR": "errors.log",
		"INFO": "info.log",
		"DEBUG": "debug.log"
	},
	"Stderr":"DEBUG",
	"Logpath": "/var/log/go-stomp-nominatim/"
}
```
но "Logpath" будет пустым (файлы с логами создаются рядом с конфигурационным файлом).<br />
Обратить внимание: программа будет работать и без логгеров, хотя и оповестит об ошибке в stderr.

b) .config<br/ >

Для go-stomp-server - логин/пароль пользователей, которые могут подключиться к серверу:

```
#cat /usr/share/go-stomp-server/go-stomp-server.config

[
        {
                "Login": "client1",
                "Passcode": "111"
        },
        {
                "Login": "client1",
                "Passcode":"111"
        }
]
```

Для go-stomp-nominatim - параметны подключения к БД Nominatim:
```
#cat /usr/share/go-stomp-nominatim/go-stomp-nominatim.config

{
        "DBname": "nominatim",
        "Host": "localhost",
        "User": "geocode1",
        "Password": "_geocode1#"
}
```

* Скрипты для демонов 

`/etc/init/go-stomp-server.conf`<br/ >
`/etc/init/go-stomp-nominatim.conf`<br/ >

Сюда можно указывать флаги подключения (флаги см. выше).

Пакет go-stomp-nominatim включает в себя пример клиента в папке /usr/share/go-stomp-nominatim/example.
Для флагов программы выполнить `./stomp-nominatim-client --help`.<br/ >
В общем говоря, это программа, которая получает координаты из файла test.csv и направляет запрос с ними на сервер

### Тестирование

* Логи<br/ >
Логи записываются с форматированием, поэтому желательно просматривать их через вывод на консоль (через cat, например).
Новое соединение определяется как ошибка (удобно отделять логи разных сессий).

* Демоны
```
sudo service go-stomp-nominatim/go-stomp-server status
sudo service go-stomp-nominatim/go-stomp-server start/stop
```
*_Notice!_ Если остановить демон сервера, go-stomp-nominatim остановится, его следует перезапустить тоже!<br/ >
Записано в (1) раздела "Доработка".*

Логи демонов:  `# sudo vim /var/log/upstart/go-stomp-server.log` 

* Пакеты:
Установить/удалить пакет:
```
# sudo dpkg [-i/-r] package.deb/package-name
```

## Дальнейшая доработка

Эти и другие будут разрабатываться после автоматизации сборки пакета.
*_Notice!_ Alpha-версия. Установка на свой страх и риск.*

1. Адрес, который слушает go-stomp-server, устанавливается демоном в 0.0.0.0:61614: заменить на localhost:61614, 
а по фелолту поставить порт 61614 вместо 61613.

2. Автоподключение go-stomp-nominatim после остановки go-stomp-server: если остановить работу последнего, 
go-stomp-server также прекращает работу и его тоже надо перезапускать (решить путем создания одного пакета/добавить зависимости).

3. Добавить `--help` флаг и запускать бинарники с ним автоматически.

4. Как-то проверять установленную БД Nominatim при установке go-stomp-nominatim?

5.```
13:28:48.285 [ERROR] github.com/go-stomp/stomp/server/client: read failed: invalid frame format: 192.168.240.105:53944,<br/ >
13:28:55.705 [ERROR] github.com/go-stomp/stomp/server/client: read failed: invalid command: 192.168.240.105:54065,<br/ >
```

Etc, etc.
