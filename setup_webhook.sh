#!/bin/bash
# Запусти один раз после деплоя на Vercel
# Заменяет polling на webhook

VERCEL_URL="https://telegram-bot-for-pubg-group.vercel.app"
TOKEN="8758844312:AAFWSnkOuj8Iekv0nFVlYKkCYH-SnLJUhfw"

curl "https://api.telegram.org/bot${TOKEN}/setWebhook?url=${VERCEL_URL}/api/webhook"
