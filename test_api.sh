#!/bin/bash

BASE_URL="http://localhost:8080"

echo "Введите имя участника:"
read NAME

echo "Введите email участника:"
read EMAIL

echo "Создаём пользователя..."
USER_RESPONSE=$(curl -s -X POST "$BASE_URL/users" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$NAME\",\"email\":\"$EMAIL\"}")

echo "Ответ сервера при создании пользователя: $USER_RESPONSE"

USER_ID=$(echo "$USER_RESPONSE" | grep -oP '(?<="id":)[0-9]+')

if [ -z "$USER_ID" ]; then
  echo "Не удалось получить user_id."
  exit 1
fi

echo "Пользователь создан с ID: $USER_ID"

echo "Введите event_id для бронирования:"
read EVENT_ID

echo "Бронируем место для пользователя (id=$USER_ID) на событие (id=$EVENT_ID)..."
BOOKING_RESPONSE=$(curl -s -X POST "$BASE_URL/bookings" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":$USER_ID,\"event_id\":$EVENT_ID}")

echo "Ответ сервера при бронировании: $BOOKING_RESPONSE"

echo "Получаем список участников события..."
PARTICIPANTS_RESPONSE=$(curl -s "$BASE_URL/events/$EVENT_ID/participants")

if [ -z "$PARTICIPANTS_RESPONSE" ]; then
  echo "Ошибка чтения данных: пустой ответ"
else
  echo "Список участников:"
  echo "$PARTICIPANTS_RESPONSE"
fi
