# Определяем пакет, к которому будут обращаться наши сервисы.
# Путь v1/data/httpapi/authz в OPA_URL как раз указывает сюда.

package httpapi.authz

# По умолчанию - запрещаем всё. Это самый безопасный подход.
default allow = false

# --- ПРАВИЛА РАЗРЕШЕНИЯ ---

# ПРАВИЛО 1: Разрешаем доступ, если у пользователя есть роль "admin".
# input.user.roles[_] == "admin" - эта конструкция проверяет,
# есть ли в массиве ролей пользователя элемент "admin".
allow {
    input.user.roles[_] == "admin"
}

# ПРАВИЛО 2: Разрешаем доступ, если у пользователя есть роль "customer"
# И при этом он пытается создать транзакцию.
allow {
    input.user.roles[_] == "customer"
    input.method == "POST"
    input.path == "/api/v1/transaction"
}

# ПРАВИЛО 3 (Пример на будущее): Разрешаем пользователю просматривать
# только свои собственные транзакции.
# allow {
#     input.method == "GET"
#     path_parts := split(input.path, "/")
#     path_parts[1] == "api"
#     path_parts[2] == "v1"
#     path_parts[3] == "transaction"
#     
#     # Сравниваем ID из пути запроса с ID пользователя из токена
#     transaction_owner_id := path_parts[4]
#     transaction_owner_id == input.user.sub
# }

# ПРАВИЛО 4: Менеджеры могут смотреть аналитику
allow {
    input.user.roles[_] == "manager"
    input.method == "GET"
    input.path == "/api/v1/analytics"
}