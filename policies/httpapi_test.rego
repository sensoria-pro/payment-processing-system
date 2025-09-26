package httpapi.authz

# Тест: убедиться, что новая роль "manager" имеет доступ
test_manager_can_access {
    # Создаем моковый "input", который придёт от нашего Go-сервиса
    mock_input := {
        "method": "GET",
        "path": "/api/v1/analytics",
        "user": {
            "sub": "user-manager-789",
            "roles": ["manager"]
        }
    }

    # Проверяем, что с этим input'ом правило "allow" вернёт true
    allow with input as mock_input
}