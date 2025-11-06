# Arxitektura va RabbitMQ ishlash printsipi (UZ)

Ushbu hujjat mikroxizmatlar arxitekturasi, RabbitMQ topologiyasi, xabar formati, event/command oqimlari va WebSocket bilan bog‘lanishni chuqur yoritadi. Maqsad — tizimga yangi kelgan muhandis RabbitMQ orqali qanday aloqalar borligini, qaysi servis nima qiladi va qayerda qanday konfiguratsiya borligini to‘liq tushunsin.


## 1. Komponentlar va roli

- API Gateway (HTTP + WebSocket)
  - HTTP REST: /trip/preview, /trip/start
  - WS: /ws/drivers, /ws/riders
  - Frontend bilan yagona kirish nuqtasi
- Trip Service (gRPC)
  - Marshrutni OSRM’dan olish, paketlar bo‘yicha narxlarni hisoblash
  - Trip yaratish va event nashr etish (trip.event.created)
- Driver Service (gRPC + AMQP consumer/publisher)
  - Haydovchilarni ro‘yxatdan o‘tkazish (API Gateway orqali gRPC)
  - trip.event.* hodisalarini iste’mol qilish; mos driver topish
  - driver.cmd.* komandalarini nashr etish (haydovchiga buyruqlar)
- RabbitMQ
  - Topic exchange: trip
  - Queue: find_available_drivers (driver-service iste’mol qiladi)
- Kelgusida: Payment Service, Notification Service


## 2. RabbitMQ topologiyasi

- Exchange: trip (topic)
- Queue: find_available_drivers
  - Binding routing keys:
    - trip.event.created
    - trip.event.driver_not_interested
- Routing key konventsiyalari (shared/contracts/amqp.go):
  - Trip eventlar: trip.event.*
    - trip.event.created, trip.event.driver_assigned, trip.event.no_drivers_found, trip.event.driver_not_interested
  - Driver komandalar: driver.cmd.*
    - driver.cmd.trip_request, driver.cmd.trip_accept, driver.cmd.trip_decline, driver.cmd.location, driver.cmd.register
  - Payment event/commandlar (reja): payment.event.*, payment.cmd.*

Nima uchun topic exchange? Chunki mavzu/routingKey bo‘yicha ko‘p turdagi hodisalarni mos queue’larga bog‘lash mumkin, servislar mustaqil kengayadi.


## 3. Xabar formati va “OwnerID” semantikasi

- AMQP xabar konverti: shared/contracts/AmqpMessage
  - OwnerID: maqsad egasi (user yoki driver). Bu identifikator Gateway/Notification Service tomonidan foydalanuvchi/ulanishni aniqlashda qo‘l keladi.
  - Data: []byte — odatda JSON encoded payload. Masalan, TripEventData { trip: pb.Trip }.

Misol (TripService → DriverService):
- routingKey: trip.event.created
- body: AmqpMessage{ OwnerID: userID, Data: json(TripEventData) }

Misol (DriverService → Driver):
- routingKey: driver.cmd.trip_request
- body: AmqpMessage{ OwnerID: driverID, Data: json(TripEventData) }

“OwnerID” ni ishlatish sababi: Gateway/Notification keyinchalik WS-ulanishlar bilan bog‘lay olishi uchun adresatsiya va filtrni soddalashtiradi.


## 4. Trip yaratish oqimi (detail)

1) User → API Gateway: POST /trip/start (tanlangan rideFareID, userID)
2) API Gateway → TripService: gRPC CreateTrip
3) TripService:
   - Fare tekshiradi va Trip yaratadi
   - trip.event.created eventini trip exchange’ga nashr etadi
4) RabbitMQ → DriverService (find_available_drivers queue’da):
   - trip.event.created keladi
   - DriverService mos haydovchilarni topadi
   - Agar yo‘q: trip.event.no_drivers_found eventini nashr etadi (OwnerID = userID)
   - Agar bor: driver.cmd.trip_request komandasi nashr etiladi (OwnerID = driverID)
5) Gateway/Notification (kelgusida):
   - no_drivers_found → Rider WS’iga yuborish
   - driver_cmd_trip_request → Driver WS’iga yuborish

Mermaid (umumiy oqim):
```mermaid
sequenceDiagram
  participant User as User
  participant APIGateway as APIGateway
  participant TripService as TripService
  participant RabbitMQ as RabbitMQ (trip)
  participant DriverService as DriverService
  participant Driver as Driver (WS)

  User ->> APIGateway: POST /trip/start
  APIGateway ->> TripService: gRPC CreateTrip
  TripService -->> RabbitMQ: trip.event.created
  RabbitMQ -->> DriverService: find_available_drivers (consume)
  alt Driver topilmadi
    DriverService -->> RabbitMQ: trip.event.no_drivers_found
  else Driver topildi
    DriverService -->> RabbitMQ: driver.cmd.trip_request (Owner=driverID)
    RabbitMQ -->> Driver: WS orqali (Gateway/Notif orqali)
  end
```


## 5. Trip preview oqimi (detail)

1) User → API Gateway: POST /trip/preview (start/end koordinatalar)
2) API Gateway → TripService: gRPC PreviewTrip
3) TripService → OSRM: marshrutni hisoblaydi
4) TripService: paketlar bo‘yicha narxlarni hisoblaydi, foydalanuvchi bilan bog‘lab vaqtincha saqlaydi
5) API Gateway: route + rideFares ni JSON qilib qaytaradi

Bu oqim RabbitMQ’ga tegmaydi, lekin CreateTrip bosqichida asinxron eventlar ishga tushadi.


## 6. Consumer semantika: QoS, Ack/Nack, Retry/DLQ

- QoS(1): har bir consumer bir vaqtning o‘zida bitta xabar oladi (fair dispatch)
- Ack: handler muvaffaqiyatli bo‘lsa qo‘l bilan Ack qilinadi
- Nack(requeue=false): xatoda qayta navbatga qo‘yilmaydi
- mandatory=true: publish qilinganda mos queue bo‘lmasa returned message log qilinadi
- Tavsiya (prod):
  - DLX/DLQ: xatolikdagi xabarlarni o‘lik navbatga yo‘naltirish
  - Retry-politikalar: oraliqli qayta urinish (exponential backoff)
  - Idempotent handlerlar: qayta ishlash xavfsiz bo‘lsin


## 7. WebSocket qatlamining o‘rni

- /ws/drivers: driver ulanadi → Gateway gRPC orqali driver-service’da RegisterDriver qiladi → WS orqali “driver.cmd.register” xabari qaytariladi
- /ws/riders: rider ulanadi → hozircha log; keyinroq notificationlar yuboriladi
- Kelajak: Gateway AMQP’dagi event/commandlarni WS’ga tarqatadi:
  - trip.event.no_drivers_found → Riderga
  - driver.cmd.trip_request → Driverga
  - driver.cmd.trip_accept/decline (Driverdan keladigan) → RabbitMQ’ga qayta publish

Shu ko‘prik tufayli mobil/web mijozlar real vaqtda trip statusini oladi.


## 8. Ma’lumot modellari (Protobuf)

- trip.proto:
  - TripService: PreviewTrip, CreateTrip
  - RideFare, Route, Geometry, Coordinate, Trip
- driver.proto:
  - DriverService: RegisterDriver, UnregisterDriver
  - Driver, Location

TripEventData (shared/messaging/events.go) → pb.Trip ni o‘z ichiga oladi. AMQP Data maydonida JSON sifatida yuboriladi.


## 9. Deploy va konfiguratsiya (K8s)

- RabbitMQ (infra/development/k8s/rabbitmq-deployment.yaml):
  - StatefulSet, readiness/liveness/startup probe’lar
  - 5672 (AMQP), 15672 (Management)
  - Credentials: secret rabbitmq-credentials orqali
- Servislar uchun env:
  - RABBITMQ_URI (amqp://user:pass@rabbitmq:5672/)
  - GRPC_ADDR har bir servisda
  - GATEWAY_HTTP_ADDR

Prod uchun tavsiyalar:
- RabbitMQ klaster/HA, persistensiya
- TLS, autentifikatsiya, tarmoq siyosatlari
- Observability: metrics, tracing (correlation-id), loglar


## 10. Xavfsizlik va barqarorlik

- Guest/guest faqat lokal dev uchun; prod’da albatta alohida user/role
- Mandatory publish + returned message logging moslanmagan routingni erta aniqlashga yordam beradi
- DLQ/Retry bilan transient xatolarni yoping, zahira monitoring qo‘ying


## 11. Kelgusidagi kengaytmalar

- Notification Service
  - Vazifa: AMQP eventlarini user/driver WebSocket ulanishlariga tarqatish
  - Bindinglar: trip.event.no_drivers_found, trip.event.driver_assigned, payment.event.session_created, payment.event.success/failed
  - Shaxsiylashtirish: OwnerID bo‘yicha WS sessiyani topish va yuborish
- Payment Service
  - Stripe checkout session yaratish (trip.event.driver_assigned’dan keyin)
  - Webhook → payment.event.success / failed → Trip status yangilash
- Driver ack oqimi
  - driver.cmd.trip_accept/decline → TripServicega qayta ishlash, status update


## 12. Amaliy tekshiruv ro‘yxati

- Exchange/queue deklaratsiyasi: trip + find_available_drivers → bindings to‘g‘rimi?
- Driver-service ishga tushganda: QoS(1) o‘rnatilganmi, Ack/Nack loglari bormi?
- CreateTrip paytida: trip.event.created publish qilinyaptimi? returned message bormi?
- Mos driver yo‘q ssenariy: trip.event.no_drivers_found yuborilyaptimi?
- WS driver ulanishi: RegisterDriver OKmi, qaytgan WS xabari (“driver.cmd.register”) kelyaptimi?


## 13. Xulosa

Tizim RabbitMQ orqali event-driven bo‘lib, Trip va Driver servislar bo‘ylab bo‘linadi. Topic exchange va aniq routing key konventsiyasi (event/command) mikroxizmatlarni qo‘shish/kengaytirishni osonlashtiradi. OwnerID kontseptsiyasi esa WS/Notification qatlamiga aniq adresatsiya olib kiradi. Kelgusida Notification va Payment servislar qo‘shilishi bilan real ish jarayoni to‘liq yopiladi va foydalanuvchiga real vaqt rejimida boy tajriba taqdim etiladi.
