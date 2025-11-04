# Tizim qanday ishlaydi (batafsil, Uzbek)

Ushbu hujjat hozirgi arxitektura, asosiy fayllar/funksiyalar va muhim mantiqlarni sodda lekin to‘liq tushuntiradi. Maqsad: “Nima qilinyapti va nega shunday qilinyapti”ni tushunish.

## 1. Katta rasm (Architecture overview)

- Frontend (Next.js) → API Gateway (HTTP) → Trip-service (gRPC) → OSRM (marshrut API)
- API Gateway frontenddan HTTP so‘rov oladi va trip-service bilan gRPC orqali gaplashadi.
- Trip-service OSRM’dan marshrutni olib, paketlar (sedan, suv, van, luxury) bo‘yicha taxminiy narxlarni hisoblaydi.
- API Gateway javobni JSON qilib frontendga qaytaradi.

Diqqat: Kubernetesda trip-service’ga `trip-service:9093` nomi/porti orqali ulaniladi.

## 2. Ma’lumot oqimi: Trip preview

1) Frontend xaritada manzil tanlaydi va POST `/trip/preview` yuboradi (API Gatewayga).
2) API Gateway JSON’ni o‘qiydi, gRPC orqali Trip-service’ning `PreviewTrip` metodini chaqiradi.
3) Trip-service OSRM’dan marshrutni oladi (yo‘l koordinatalari, masofa, vaqt).
4) Trip-service paketlar bo‘yicha narxlarni hisoblaydi va vaqtincha saqlaydi (in-memory).
5) Trip-service marshrut + rideFares ro‘yxatini qaytaradi.
6) API Gateway javobni `{ data: { route, rideFares } }` formatida frontendga beradi.

## 3. Muhim fayllar va vazifalari

### 3.1. API Gateway
- `services/api-gateway/main.go`
  - HTTP serverni ishga tushiradi (`GATEWAY_HTTP_ADDR`, default `:8081`).
  - Marshrutlar: `POST /trip/preview`, `GET /ws/drivers`, `GET /ws/riders`.
- `services/api-gateway/http.go`
  - `handleTripPreview`: kelgan JSON’ni parse qiladi, `NewTripServiceClient()` bilan gRPC ulanishini ochadi, `PreviewTrip` RPC’ni chaqiradi, natijani `contracts.APIResponse` ga o‘rab qaytaradi.
- `services/api-gateway/types.go`
  - `previewTripRequest` (frontenddan keladigan schema). `ToProto()` bilan gRPC requestga aylantiradi.
- `services/api-gateway/grpc_clients/trip_client.go`
  - `NewTripServiceClient()`: `TRIP_SERVICE_URL` (default `trip-service:9093`) ga gRPC dial qiladi, `pb.TripServiceClient` qaytaradi.
- `services/api-gateway/middleware.go`
  - `enableCors`: CORS uchun sarlavhalar, `OPTIONS` ga 204.

### 3.2. Trip-service
- `services/trip-service/cmd/main.go`
  - gRPC serverni ishga tushiradi (`GRPC_ADDR`, default `:9093`).
  - In-memory repository va service qatlamini yaratadi.
- `services/trip-service/internal/infrastructure/grpc/grpc_handler.go`
  - gRPC servis registratsiyasi.
  - `PreviewTrip(ctx, req)`: koordinatalarni oladi, `service.GetRoute()` chaqiradi, `EstimatePackagesPriceWithRoute()` bilan narxlarni hisoblaydi, `GenerateTripFares()` bilan ID/UserID berib saqlaydi, `pb.PreviewTripResponse` qaytaradi.
- `services/trip-service/internal/service/service.go`
  - `GetRoute(ctx, pickup, destination)`: OSRM’ga HTTP GET, JSON’ni `pkg/types.OsrmApiResponse` ga parse qiladi.
  - `EstimatePackagesPriceWithRoute(route)`: bazaviy paketlar ro‘yxati (sedan, suv, van, luxury) bo‘yicha bahoni taxmin qiladi.
  - `GenerateTripFares(ctx, fares, route, userID)`: har bir hisoblangan fare uchun yangi ID va `userID` qo‘yib, repo’ga saqlaydi va ro‘yxatni qaytaradi.
  - `estimateFareRoute(f, route)`: narx formulasi (quyida qarang).
  - `getBaseFares()`: paketlar uchun boshlang‘ich narxlar (slug: sedan/suv/van/luxury).
- `services/trip-service/internal/infrastructure/repository/inmem.go`
  - Oddiy xotira (map) ichida `TripModel` va `RideFareModel` saqlanadi.
- `services/trip-service/internal/domain/ride_fare.go`
  - `RideFareModel` → `pb.RideFare` ga o‘tkazish (`ToProto`), hamda massivni o‘tkazish (`ToRideFaresProto`).
- `services/trip-service/internal/domain/trip.go`
  - Domen interfeyslari: `TripRepository`, `TripService` (Preview oqimi uchun kerak bo‘lgan signaturalar).
- `services/trip-service/pkg/types/types.go`
  - OSRM javobi modeli (`OsrmApiResponse`).
  - `ToProto()`: OSRM marshrutini `pb.Route` ga aylantiradi.
  - `DefaultPricingConfig()`: narx formulasi uchun konfiguratsiya.

### 3.3. Shared
- `shared/proto/trip/*.go`
  - gRPC protobuf turlari: `PreviewTripRequest/Response`, `Route`, `RideFare`, `Coordinate` va h.k.
- `shared/contracts/http.go`
  - API Gateway javobi qadoqlash: `APIResponse{ data, error }`.

## 4. Muhim mantiqlar (Why/How)

### 4.1. OSRM koordinatalari va frontendga moslashtirish
- OSRM koordinata massivi: `[lon, lat]` (ya’ni `[uzunlik, kenglik]`).
- Frontend polyline chizishda serverdan kelgan obyektni `[coord.longitude, coord.latitude]` ga o‘giradi.
- Shuning uchun backend `pb.Coordinate` ni quyidagicha to‘ldiradi:
  - `Latitude  = coord[0]` (ya’ni lon)
  - `Longitude = coord[1]` (ya’ni lat)
- Nega bunday? Frontenddagi mavjud mantiq o‘zgarmaydi, ammo natija polyline to‘g‘ri chiziladi. Bu “swap” frontdagi parsingga moslashtirilgan qaror.

### 4.2. Narx hisoblash formulasi
- Konfiguratsiya: `DefaultPricingConfig()` → `PricePerDistance`, `PricingPerMinute`.
- Hisoblash: 
  - `distanceFare = route.Distance * PricePerDistance`
  - `timeFare     = route.Duration * PricingPerMinute`
  - `carBase      = f.TotalPriceCents` (paketga bog‘liq)
  - `totalPrice   = carBase * timeFare + distanceFare`
- Nega bunday? Soddalashtirilgan model: vaqt+masofa+paket bazasi orqali o‘rta hisob narx.

### 4.3. Paket sluglari muhim
- `van` to‘g‘ri; `vam` → noto‘g‘ri.
- Frontend `PackagesMeta` lug‘atida `van` bor, `vam` bo‘lsa `icon` topilmaydi va front xato beradi.

### 4.4. gRPC handler’dagi xatolar
- Ilgari `log.Fatalf` ishlatilgan joy servisni yiqitardi.
- Endi `log.Printf` + `status.Error(...)` bilan xato qaytariladi, servis ishlashda davom etadi.

### 4.5. Interfeys signaturalari uyg‘unligi
- `EstimatePackagesPriceWithRoute` → endi `[]*RideFareModel` qaytaradi (chunki bir nechta paket uchun baho). 
- `GenerateTripFares(ctx, fares, route, userID)` → route va userID bilan saqlash.
- Nega bunday? Type mos kelmaslik va “slice vs single item” xatolarini bartaraf etadi.

## 5. Konfiguratsiya va portlar

- API Gateway: `GATEWAY_HTTP_ADDR = :8081` (K8s Service ham 8081)
- Trip-service: `GRPC_ADDR = :9093` (K8s Service ham 9093)
- API Gateway gRPC target: `TRIP_SERVICE_URL=trip-service:9093` (default shu)
- Nega bunday? K8s ichida servis nomi bilan tarmoqga chiqish – barqaror va standard yondashuv.

## 6. Tipik muammolar va sabablari

- 502 Bad Gateway: Trip-service yiqilib qolgan (fatal log), yoki gRPC manzil noto‘g‘ri.
- Front `icon` xatosi: slug `vam` → `van` bo‘lishi kerak.
- Chiziq chizilmasligi: koordinata mapping front kutgan formatdan farq qiladi; backenddagi “swap” muvofiqlashtiradi.
- `Cannot use 'estimatedFares' ... as []*RideFareModel`: interfeys imzosi/return turi mos emas; `[]*RideFareModel` qilib berish kerak.

## 7. Fayl/funksiya – “nima uchun bor?” tezkor ro‘yxat

- API Gateway
  - `main.go`: server, marshrutlar.
  - `http.go` → `handleTripPreview`: JSON kirish → gRPC chaqiriq → JSON chiqish.
  - `types.go` → `previewTripRequest.ToProto()`: HTTP → gRPC request mapping.
  - `grpc_clients/trip_client.go` → gRPC ulanish (service discovery `trip-service:9093`).

- Trip-service
  - `cmd/main.go`: gRPC server bootstrap.
  - `infrastructure/grpc/grpc_handler.go` → `PreviewTrip`: barcha biznes oqimlarni servislarga ulaydi.
  - `service/service.go`:
    - `GetRoute`: OSRM’dan marshrut oladi.
    - `EstimatePackagesPriceWithRoute`: paketlar bo‘yicha baho taxmini.
    - `GenerateTripFares`: saqlash uchun ID+userID berish.
    - `estimateFareRoute`: narx formulasi.
  - `infrastructure/repository/inmem.go`: vaqtinchalik saqlash.
  - `domain/ride_fare.go`: proto mapping.
  - `pkg/types/types.go`: OSRM modeli, `ToProto()`, pricing config.

## 8. Yakuniy oqim (qisqa)

Frontend → `/trip/preview` → API Gateway → gRPC `PreviewTrip` → Trip-service:
- OSRM’dan route
- paketlar bo‘yicha narx
- fares’ni saqlash (ID, userID)
- route + rideFares qaytadi → API Gateway → Frontend.

Koordinatalar backendda moslashgan, shuning uchun frontenddagi mavjud transformatsiya bilan chiziq to‘g‘ri chiziladi. Paket sluglari to‘g‘ri (`van`), shuning uchun ikonlar chiqa oladi. gRPC handler endi servisni yiqitmaydi, xatolar status sifatida qaytadi.
