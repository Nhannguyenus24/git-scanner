# Sử dụng GitScanner

GitScanner clone toàn bộ lịch sử của một kho mã, đọc mỗi phiên bản file duy nhất
(blob) đúng một lần, và báo cáo các credential tìm được — trong cây hiện tại lẫn
bất kỳ đâu trong quá khứ. Một key đã xóa từ ba năm trước vẫn đọc được bởi bất kỳ
ai clone kho, nên nó vẫn là rò rỉ cho tới khi bị thu hồi.

Hướng dẫn này đi qua từng trang của bảng điều khiển. Bản rút gọn:

1. Kiểm tra chấm cạnh địa chỉ API ở thanh trên cùng đang xanh.
2. Bấm **Quét mới** và nhập một kho, một người dùng hoặc một tổ chức.
3. Theo dõi trang của lượt quét đến khi xong.
4. Xử lý ở trang **Phát hiện**: xoay khóa những key còn sống, đánh dấu những
   gì không phải rò rỉ.
5. Báo cho chủ sở hữu bằng mail thông báo từ trang **Kho mã**.

---

## Màn hình

### Thanh trên cùng

- **Địa chỉ API** — máy chủ cung cấp mọi dữ liệu trên màn hình. Chấm bên cạnh
  màu xám khi đang kiểm tra, xanh khi API trả lời, đỏ khi không tới được. Nhập
  host của bạn nếu không chạy ở mặc định `http://localhost:8080`; địa chỉ được
  nhớ trong trình duyệt này.
- **Ô lệnh** — mở bảng lệnh (xem bên dưới).
- **Trợ giúp** — trang này.
- **Sáng / tối** — chuyển giữa nền giấy sáng và tối.
- **Ngôn ngữ** — English, Tiếng Việt, Italiano, 中文, Español, Deutsch và
  Français. Mọi thứ, kể cả hướng dẫn này, đổi theo.

### Thanh bên

Mọi trang đều cách một cú bấm. Ba mục có số đếm trực tiếp: **Phát hiện** (phát
hiện đang mở), **Lượt quét** (lượt quét đang chạy) và **Chưa gắn** (dạng key
đang chờ luật nhà cung cấp). Thu gọn thanh bên chỉ còn biểu tượng bằng mũi tên ở
cuối thanh hoặc `Ctrl+[` (`⌘[` trên Mac). Trên điện thoại, thanh bên chuyển thành
thanh dưới đáy; các trang ít dùng nằm trong **Thêm**.

### Phím tắt

- `Ctrl+K` / `⌘K` — mở hoặc đóng bảng lệnh, ở bất kỳ đâu.
- `/` — mở bảng lệnh.
- `N` — bắt đầu lượt quét mới.
- `Ctrl+[` / `⌘[` — thu gọn hoặc mở rộng thanh bên.
- `Esc` — đóng hộp thoại, ngăn chi tiết hoặc bảng lệnh.

`N` và `/` bị bỏ qua khi bạn đang gõ trong ô nhập, nên không bao giờ "ăn" phím
dành cho bộ lọc.

### Bảng lệnh

Một ô cho mọi thứ bảng điều khiển làm được bằng bàn phím. Gõ để thu hẹp danh
sách, di chuyển bằng `↑` `↓` (hoặc `Ctrl+N` / `Ctrl+P`), chạy bằng `Enter`. Gồm:

- **Hành động** — bắt đầu quét mới; xuất khung nhìn hiện tại.
- **Đi tới** — mọi trang.
- **Bộ lọc** — các bộ lọc phát hiện có sẵn và bộ lọc bạn đã lưu, áp dụng một
  bước.
- **Giao diện** — sáng hoặc tối, theo hệ thống, từng chủ đề, mật độ, chuyển
  động, thanh bên và đặt lại.
- **Ngôn ngữ** — đổi ngôn ngữ giao diện.

Mọi nội dung khác bạn gõ trở thành **Tìm phát hiện với “…”**, mở trang Phát hiện
với chuỗi đó làm bộ lọc tìm kiếm. Bảng lệnh cố ý không lưu lịch sử: ô tìm kiếm có
trí nhớ là nơi một credential lỡ dán vào có thể nằm lại.

### Phần giới thiệu

Lần đầu mở, bảng điều khiển đưa ra phần giới thiệu bảy bước. Có thể bỏ qua bất
cứ lúc nào và xem lại sau ở **Cài đặt → Phần giới thiệu**.

---

## Bắt đầu một lượt quét

Bấm **Quét mới** (thanh bên, trang Tổng quan hoặc Lượt quét, hoặc phím `N`). Chọn
mục tiêu:

- **Kho** — một URL clone HTTPS, ví dụ `https://github.com/owner/repo`. URL SSH
  và URL chứa tên người dùng hoặc token đều bị từ chối.
- **Người dùng** — mọi kho mà một người dùng GitHub sở hữu. Tên tài khoản gồm
  chữ, số và gạch nối, 1–39 ký tự.
- **Tổ chức** — mọi kho mà một tổ chức GitHub sở hữu.
- **Ngẫu nhiên** — một mẫu ngẫu nhiên các kho công khai, chỉ để kiểm thử dò
  quét. Nhập số lượng, hoặc tích **Chạy mãi** để lấy mẫu liên tục đến khi tạm
  dừng hoặc hủy.

Với Người dùng, Tổ chức và Ngẫu nhiên, tích **Gồm cả kho fork** để quét cả fork;
mặc định fork bị bỏ qua vì phần lớn lặp lại kho gốc.

Bấm **Bắt đầu quét** hoặc `Enter`. Lượt quét được xếp hàng chạy nền và bảng điều
khiển mở trang chi tiết của nó.

---

## Theo dõi lượt quét

### Danh sách lượt quét

**Lượt quét** liệt kê mọi job API đã nhận, mới nhất trước. Chuyển giữa **Tất
cả**, **Đang chạy** và **Đã xong**. Mỗi dòng hiện trạng thái, mục tiêu, tiến độ,
số blob đã đọc, phát hiện mới, thời điểm bắt đầu và thời lượng. Bấm vào dòng để
mở; biểu tượng ở cuối dòng dùng để quét lại hoặc xóa job đã xong.

Một job ở một trong sáu trạng thái: **chờ**, **đang chạy**, **tạm dừng**, **hoàn
tất**, **lỗi** hoặc **đã hủy**.

### Chi tiết lượt quét

Phần đầu ghi mục tiêu, loại quét, lúc gửi và đã chạy bao lâu. Các nút thay đổi
theo trạng thái:

- **Tạm dừng** (đang chạy) — worker ngừng nhận kho mới; những gì đã tìm thấy
  không mất. **Tiếp tục** chạy tiếp từ chỗ đã dừng.
- **Hủy** (job chưa xong) — dừng job hẳn.
- **Quét lại** (đã xong) — xếp hàng lại cùng mục tiêu thành một job mới.
- **Xóa** (đã xong) — xóa job **cùng mọi phát hiện nó đã ghi**. Không thể hoàn
  tác.

Bên dưới:

- **Bộ đếm** — số kho xong trên tổng, số blob đã quét, và phát hiện mới (lần đầu
  thấy bởi job này).
- **Tiến độ và thời gian còn lại** — ước lượng hiện ra khi đủ dữ liệu; trước đó
  hãy tin vào bộ đếm.
- **Giai đoạn** — Tìm kho → Quét kho (clone, duyệt lịch sử, dò quét) → Phân loại
  nhà cung cấp → Hoàn tất phân tích.
- **Hoạt động trực tiếp** — được worker truyền về khi lượt quét đang chạy.
- **Tóm tắt** — khi job hoàn tất: bao nhiêu phát hiện trong bao nhiêu kho, bao
  nhiêu ở mức cao hoặc nghiêm trọng, bao nhiêu vẫn lấy được từ nhánh mặc định, và
  những kho bị ảnh hưởng.
- **Phát hiện ghi bởi lượt quét này** — chính danh sách đó, kèm liên kết mở ở
  trang Phát hiện.

### Hoạt động trực tiếp

Bảng trực tiếp cho thấy từng worker đang làm gì. Trạng thái của nó là **đang kết
nối**, **trực tiếp**, **đang kết nối lại**, **đã kết thúc** hoặc **không khả
dụng**; nếu máy chủ không hỗ trợ truyền trực tiếp, các bộ đếm phía trên vẫn tự
làm mới vài giây một lần.

- **Khám phá** — với lượt quét Người dùng và Tổ chức, việc liệt kê các kho của
  tài khoản theo từng trang. *Chờ giới hạn tốc độ của GitHub* là tạm nghỉ, không
  phải lỗi.
- **Đang xử lý** — từng kho và bước đang chạy: **clone** (bare: chỉ nhánh và
  tag), **đọc lịch sử** (mọi phiên bản file duy nhất, kèm số khớp ứng viên đang
  tăng), **truy commit** (lần ra commit đã đưa từng kết quả khớp vào), **kiểm tra
  key** (kiểm tra còn sống, nếu được bật) và **lưu**.
- **Vừa xong** — các kho vừa hoàn tất, với số phát hiện mới, số blob đã đọc và
  thời gian.
- **Nhật ký hoạt động** — toàn bộ dòng sự kiện; **Xem thêm** để mở rộng.

Kho nào thất bại một lần thử sẽ tự được thử lại và hiện *lần thử 2*, *lần thử
3*… Kho thất bại ở lần thử cuối bị **bỏ qua**; phần còn lại của job vẫn chạy
tiếp.

### Vì sao lượt quét không tìm thấy gì mới

Một credential chỉ được ghi một lần, bởi lượt quét đầu tiên tìm ra nó. Quét lại
cùng kho sẽ thấy lại các key đó nhưng không ghi phát hiện *mới* — chúng vẫn gắn
với lượt quét trước. Mở **Phát hiện** lọc theo kho đó để xem đầy đủ.

---

## Tổng quan

Trang tiêu điểm, tự làm mới mỗi mười giây (dấu **Trực tiếp** hiện chu kỳ).

- **Tình hình an ninh** — điểm trên thang 100. Bấm **Cách tính** để xem từng
  khoản trừ (bên dưới).
- **Số liệu chính** — phát hiện đang mở, nghiêm trọng, còn ở HEAD, kho bị lộ,
  lượt quét đang chạy, dạng chưa gắn.
- **Mở theo mức độ**, **Mở theo nhà cung cấp**, **Phân loại** — các biểu đồ phân
  tách. Mọi ô và thanh đều là liên kết tới danh sách Phát hiện đã lọc.
- **Hoạt động gần đây** — lượt quét xong hoặc lỗi, phát hiện mới, thông báo đã
  gửi.
- **Lượt quét gần đây** — các job mới nhất và tiến độ.

### Bố cục

**Bố cục** chuyển giữa bốn cách sắp xếp: **Tất cả**, **Xử lý hằng ngày** (số liệu
và phân loại, không điểm), **Tóm tắt điều hành** (điểm số, con số chính, thay đổi
gần đây) và **Vận hành an ninh** (ưu tiên hoạt động và phiên quét đang chạy).
**Tùy chỉnh** cho phép đưa bảng lên hoặc xuống bằng mũi tên, ẩn bằng biểu tượng
con mắt, và đưa bảng đã ẩn trở lại từ danh sách bên dưới. Bố cục được lưu trong
trình duyệt này.

### Cách tính điểm tình hình an ninh

Điểm bắt đầu từ 100 và bị trừ theo từng yếu tố. Mỗi khoản trừ đều bão hòa: vài
phát hiện đầu tiên trừ nhiều nhất và không yếu tố nào trừ vô hạn.

- **Phát hiện nghiêm trọng** — tối đa 30 điểm.
- **Có thể lấy từ HEAD** — tối đa 25 điểm. Tín hiệu sắc nhất: ai clone nhánh mặc
  định hôm nay cũng lấy được credential.
- **Phát hiện mức cao** — tối đa 15 điểm.
- **Kho bị lộ** — tối đa 15 điểm, theo tỉ lệ kho đã quét còn phát hiện mở.
- **Tỉ lệ chưa xử lý** — tối đa 10 điểm, theo tỉ lệ phát hiện còn mở. Phát hiện
  đã đóng không bị trừ.
- **Chủ sở hữu chưa được báo** — tối đa 5 điểm, cho các kho bị lộ chưa được gửi
  thông báo.

Từ 85 trở lên là **Ổn định**, 65–84 **Cần cải thiện**, 40–64 **Đang lộ**, dưới 40
**Nghiêm trọng**. Khi chưa quét gì thì không có điểm, thay vì một con số 100 gây
hiểu nhầm. Điểm được tính trong trình duyệt từ bộ đếm toàn cơ sở dữ liệu của máy
chủ, không bao giờ từ một trang phát hiện.

---

## Phát hiện

Danh sách đầy đủ mọi credential đã ghi nhận, 50 dòng mỗi trang.

### Lọc

- **Tìm kiếm** — đường dẫn, luật, nhà cung cấp hoặc kho.
- **Mức độ**, **Nhà cung cấp**, **Trạng thái** (Đang mở, Đã xoay khóa, Báo nhầm,
  Bỏ qua).
- **Mức lộ** — ở HEAD, chỉ trong lịch sử, hoặc xác nhận còn sống.
- **Sắp xếp** — mặc định, phát hiện mới nhất, commit cũ nhất, hoặc mức độ.
- **Một kho** / lượt quét — hiện dưới dạng chip khi bạn đến từ một kho hoặc một
  lượt quét; bỏ chip để xem tất cả.

Bộ lọc nằm trong URL, nên khung nhìn đã lọc có thể đánh dấu, chia sẻ hoặc mở lại
sau khi tải lại trang. **Xóa bộ lọc** để đặt lại.

Mức lộ và Sắp xếp được xử lý trong trình duyệt. Khi bật một trong hai, trang tải
tối đa 500 phát hiện từ máy chủ rồi lọc trên đó; nếu có nhiều hơn, trang ghi rõ
phía trên bảng thay vì hiện một tổng số sai.

### Bộ lọc có sẵn và bộ lọc đã lưu

Menu **Bộ lọc** có các khung nhìn dựng sẵn:

- **Nghiêm trọng, còn mở** — xoay vòng các khóa này trước.
- **Có thể lấy từ HEAD** — ai clone hôm nay cũng đọc được.
- **Xác nhận còn sống** — thăm dò nhận phản hồi hợp lệ.
- **Chưa gán nhà cung cấp** — chưa luật nhà cung cấp nào nhận.
- **Mới phát hiện** — phát hiện mới nhất trước.
- **Tồn đọng lâu nhất** — commit cũ nhất trước.

**Lưu bộ lọc hiện tại** lưu bộ lọc bạn đang đặt dưới một cái tên; có thể đổi tên
hoặc xóa ngay trong menu đó, và nó cũng xuất hiện trong bảng lệnh. Bộ lọc đã lưu
chỉ nằm trong trình duyệt này và chỉ giữ các trường lọc — không bao giờ giữ một
chuỗi tìm kiếm trông như secret bị dán vào.

### Đọc một dòng

- **Mức độ** và **nhà cung cấp** sở hữu credential (hoặc *chưa gắn*).
- **Credential** — bản xem trước đã che. Giá trị thật không bao giờ rời engine
  trừ khi bạn chủ động bật bỏ che (xem Cài đặt).
- **Kho** và **Vị trí** — file và dòng, liên kết tới file trên GitHub.
- **Mức lộ** — `ở HEAD` nghĩa là vẫn còn trong cây hiện tại; `lịch sử` nghĩa là
  đã xóa nhưng vẫn lấy được khi clone. *Chưa rõ commit — quét lại để gắn* nghĩa
  là chưa lần ra commit đã đưa nó vào; quét lại sẽ bổ sung.
- **Còn sống — xoay khóa** — kiểm tra còn sống xác nhận key vẫn dùng được.
- **Trạng thái** và thời điểm **Phát hiện lúc**.

Menu ở cuối dòng đánh dấu phát hiện là **Đang mở**, **Đã xoay khóa**, **Báo
nhầm** hoặc **Bỏ qua**.

### Chi tiết phát hiện

Bấm vào một dòng để mở ngăn chi tiết. Địa chỉ của nó được đưa vào URL, nên **Sao
chép liên kết** cho đồng nghiệp đúng khung nhìn đó; **Sao chép tham chiếu** sao
chép một mã tham chiếu ngắn để dán vào ticket. Ngăn chi tiết gồm:

- **Credential** — bản xem trước đã che, nhóm và kết quả kiểm tra live. Khi giá
  trị được giữ cho một owner được bỏ che và tính năng che đang tắt, **Hiện** sẽ
  hiển thị nó, kèm nhãn **ĐANG HIỆN**.
- **Hạn mức & giới hạn** — những gì nhà cung cấp trả về khi kiểm tra còn sống:
  số request hoặc token còn lại, scope, quốc gia, tiền tệ. Là ảnh chụp tại thời
  điểm quét.
- **Vì sao bị phát hiện** — luật, nhà cung cấp được gán, độ dài và bộ ký tự, biến
  hoặc host gần đó, và mức lộ. Lời giải thích chỉ dùng hình dạng, độ dài và ngữ
  cảnh, không bao giờ dùng giá trị.
- **Vị trí** — file, commit, tác giả và ngày commit, kèm liên kết tới dòng trên
  GitHub.
- **Dòng thời gian** — đã commit, phát hiện, đã thăm dò và các quyết định xử lý.
- **Cần làm gì** — các bước khắc phục theo thứ tự: thu hồi và cấp lại ở nhà cung
  cấp, kiểm tra nhật ký audit, xóa giá trị khỏi HEAD, cân nhắc viết lại lịch sử,
  và thêm kiểm tra secret ở pre-commit.
- **Cùng kho** và **Cùng nhà cung cấp** — các phát hiện liên quan, chỉ một cú
  bấm.
- **Metadata** — mọi thông tin khác engine đã ghi.

### Xử lý hàng loạt

Tích ô đầu dòng, hoặc ô ở tiêu đề để chọn mọi dòng trên trang. Một thanh hiện ra
ở đáy với số lượng đã chọn và bốn trạng thái. **Đã xoay khóa** và **Báo nhầm**
hỏi xác nhận trước, vì mỗi cái là một khẳng định về thế giới bên ngoài — rằng key
đã bị thu hồi, rằng kết quả khớp là sai — và khi làm hàng loạt, đó là khẳng định
có thể lặng lẽ đóng một rò rỉ thật. Đang mở và Bỏ qua áp dụng ngay. **Xuất mục đã
chọn** chỉ xuất các dòng đã tích.

### Xuất dữ liệu

**Xuất** ghi ra các phát hiện đang tải, kèm bộ lọc đã tạo ra chúng trong file:

- **Phát hiện dạng CSV** — mỗi phát hiện một dòng, cho bảng tính.
- **Phát hiện dạng JSON** — có cấu trúc, cho script.
- **Báo cáo sự cố** — Markdown, nhóm theo kho.
- **Tóm tắt điều hành** — một trang, không bảng.

Bản xuất chỉ chứa bản xem trước đã che; credential dạng rõ không bao giờ được ghi
ra file, ở bất kỳ định dạng nào.

---

## Kho mã

Mọi kho đã quét, xếp hạng theo mức độ nghiêm trọng của phần lộ còn mở.

- **Lọc theo chủ/tên**, và **Chỉ kho có phát hiện mở**.
- **Sắp xếp theo** rủi ro, phát hiện đang mở, lần quét gần nhất hoặc tên.
- Các cột: loại, số phát hiện mở theo từng mức độ, mở / tổng, **Rủi ro**, **Bước
  tiếp theo**, **Đã thông báo** và quét lần cuối.
- Bấm vào một kho để mở các phát hiện của nó.

**Rủi ro** chạy từ 0 (không còn gì mở) đến 100. Phát hiện mở được tính trọng số
theo mức độ — một nghiêm trọng bằng mười lần một thấp — và tổng bão hòa, nên một
kho chỉ có một phát hiện nghiêm trọng đã xếp hạng cao. Các mức: Thấp, Trung bình
(từ 25), Cao (từ 50), Nghiêm trọng (từ 75). Rê chuột để xem số liệu phía sau.

**Bước tiếp theo** nêu một việc đáng làm nhất: *Xoay vòng khóa nghiêm trọng*,
*Báo cho chủ sở hữu*, *Thông báo đang chờ gửi*, *Gửi thông báo lỗi — thử lại*,
*Rà soát phát hiện còn lại* hoặc *Không còn gì mở*.

### Báo cho chủ sở hữu

Chip **Gửi mail thông báo** ở đầu trang cho biết mail đang bật hay tắt; nó liên
kết tới Cài đặt. Khi mail bật, nút **Gửi thông báo** trên một dòng sẽ xếp hàng
một mail gửi chủ sở hữu kho, liệt kê các phát hiện với bản xem trước đã che. Cột
**Đã thông báo** sau đó hiện *đang gửi*, *đã gửi*, *lỗi* hoặc *đã từ chối nhận*.

---

## Chưa gắn

Danh sách tồn đọng các secret entropy cao mà chưa luật nhà cung cấp nào nhận, gom
theo hình dạng. Mỗi dòng có **tên đoán** (lấy từ biến hoặc khóa gần nhất — đôi
khi chỉ là một từ gần đó), **mẫu** đã che, **hình dạng** (độ dài và bộ ký tự),
**ngữ cảnh** nơi tìm thấy, **số lần** gặp, trạng thái và lần cuối thấy. Một hình
dạng xuất hiện liên tục là một nhà cung cấp đáng viết luật. Những phát hiện loại
này hiện là *chưa gắn* cho tới khi có luật nhận; khi đó lượt quét kế tiếp sẽ gán
lại chúng.

---

## Nhà cung cấp

Mọi nhà cung cấp GitScanner có thể phát hiện và gán, nhóm theo loại (AI, cloud,
dữ liệu, thanh toán, nhắn tin, DevOps, công cụ phát triển, khác), kèm số luật của
mỗi bên. Bấm vào một nhà cung cấp để xem:

- **Vì sao ở mức độ này** — nghiêm trọng là toàn quyền; cao là truy cập API rộng;
  trung bình là quyền hẹp hoặc gắn với một host; thấp chỉ định danh tài khoản
  nhưng không hành động được trên nó.
- **Rò rỉ cho phép kẻ khác làm gì** — tác động theo từng nhóm.
- **Key trông như thế nào và được phát hiện ra sao**, mỗi luật một thẻ:
  - *tiền tố riêng* — giá trị bắt đầu bằng tiền tố của nhà cung cấp, bản thân nó
    đã là bằng chứng;
  - *tên cấu hình* — giá trị không có hình dạng riêng, chỉ báo khi được gán cho
    một cấu hình mang tên nhà cung cấp;
  - *URL kết nối* — URL có credential bên trong; URL chỉ trỏ tới localhost hoặc
    tên service compose được coi là cấu hình dev và bị bỏ;
  - *khối PEM* — private key từ dòng BEGIN tới END;
  - *chỉ theo cấu trúc* — độ dài cố định, dấu phân cách hoặc ký hiệu nhúng, được
    giữ thật hẹp có chủ đích.
  Mỗi thẻ có thể hiện cấu trúc của key, mẫu regex, đoạn chữ file bắt buộc chứa,
  và từ khóa phải xuất hiện gần đó.
- **Sau khi khớp** — giá trị giữ chỗ (`your-…-key`, `xxxx`, `changeme`), key
  trong tài liệu và sandbox, và dữ liệu test giả đều bị loại; phần còn lại chỉ
  được lưu dưới dạng fingerprint và bản xem trước đã che.

Một nhà cung cấp hiển thị luật nghiêm trọng nhất của nó. Trên từng phát hiện, mức
độ tăng một bậc nếu rò rỉ vẫn lấy được từ HEAD, và thêm một bậc với nhà cung cấp
cloud và thanh toán, tối đa là nghiêm trọng.

---

## Phân tích

Bức tranh về mọi thứ đã quét:

- **Phát hiện** và **Kho mã** — tổng số, đang mở, bị lộ trên đã quét.
- **Dòng thời gian rò rỉ** — phát hiện theo tháng credential được commit.
- **Mở theo mức độ**, **Mở theo nhà cung cấp** (nhà cung cấp có tên và nhóm chưa
  gắn), **Theo loại ứng dụng**, **Trạng thái phân loại**.
- **Luật phát hiện** — luật nào kích hoạt nhiều nhất.
- **Kiểm tra credential còn sống** — còn sống, đã thu hồi, không có probe, kiểm
  tra lỗi.
- **Mail thông báo** — số kho đã liên hệ, và số kho bị lộ chưa được thông báo.
- **Kho lộ nhiều nhất** — theo cùng điểm rủi ro với trang Kho mã.
- **Tồn đọng lâu nhất** — phát hiện đang mở theo ngày commit.

Số liệu tính trên toàn bộ thời gian; không có khoảng ngày vì API chưa hỗ trợ. Bộ
lọc ở đầu trang quyết định các liên kết tới trang Phát hiện mà trang này mở ra.

---

## Mail thông báo (Thông báo)

Mọi mail thông báo đã gửi chủ sở hữu kho và kết quả: kho, người nhận, trạng thái
(*đang gửi*, *đã gửi*, *lỗi*, *đã từ chối nhận*), số phát hiện, lúc yêu cầu và lúc
gửi. Tìm theo kho hoặc người nhận.

- **Xem mail** — nội dung dạng HTML và dạng text, dựng lại từ các phát hiện như
  hiện tại.
- **Gửi lại** — xếp hàng gửi lần nữa.

---

## Cài đặt

### Giao diện

Chỉ lưu trong trình duyệt này.

- **Chủ đề** — Giấy (giấy cũ, xanh dầu và đỏ huyết dụ), Nửa đêm (nền gần đen kiểu
  terminal, xanh lân quang), Đại dương (navy, lơ và lam), Tín hiệu (than chì, chỉ
  cảnh báo mới có màu), Tối giản (kiểu doanh nghiệp đơn giản).
- **Sáng hoặc tối** — Sáng, Tối, hoặc Hệ thống để theo hệ điều hành.
- **Mật độ** — Gọn giúp hiện nhiều dòng hơn.
- **Chuyển động** — Giảm để tắt hoạt ảnh.
- **Mặc định thu gọn thanh bên**.
- **Đặt lại giao diện** — về mặc định.

### Phần giới thiệu

**Xem lại phần giới thiệu** mở lại tour hướng dẫn.

### Gửi mail thông báo

Công tắc tổng. Khi bật, Gửi thông báo sẽ gửi mail tới chủ sở hữu kho. Khi tắt,
không gì được gửi và mọi mail đang xếp hàng bị bỏ. Hiện chưa có tùy chọn theo mức
độ hay bản tóm tắt định kỳ.

### Che secret và owner được bỏ che

**Bật** (mặc định): mọi credential đều hiện ở dạng che. **Tắt**: credential tìm
thấy trong kho của các owner ở danh sách **Owner được bỏ che** hiện đầy đủ, để
kiểm thử; mọi người khác vẫn bị che, còn mail và bản xuất luôn che.

- Chỉ thêm người dùng hoặc tổ chức GitHub mà bạn sở hữu.
- Giá trị được giữ, ở dạng mã hóa, kể từ lượt quét kế tiếp — quét lại các kho của
  owner để xem phát hiện cũ ở dạng rõ.
- Xóa một owner sẽ xóa mọi thứ đã giữ cho owner đó.
- Nếu máy chủ không có `SECRET_SEAL_KEY`, không thể bỏ che và không giá trị nào
  được giữ. Tạo bằng `openssl rand -hex 32`, thêm vào `.env.local` rồi khởi động
  lại.

Khi tắt che, một cảnh báo đỏ luôn hiện trên trang: credential đầy đủ của các owner
trong danh sách hiện cho bất kỳ ai mở được bảng điều khiển này.

---

## Kiểm tra còn sống

Khi người vận hành bật (`PROBE_SECRETS`) cho mục tiêu của chính họ, GitScanner
gọi một lệnh định danh chỉ-đọc cho mỗi phát hiện để xem key còn dùng được không và
thuộc tài khoản nào:

- **Còn sống — xoay khóa** — nhà cung cấp chấp nhận. Xoay khóa này trước tiên.
- **Đã thu hồi** — nhà cung cấp từ chối; đã được xoay hoặc thu hồi.
- **Không có probe** — chưa có kiểm tra cho nhà cung cấp này.
- **Kiểm tra lỗi** — giới hạn tốc độ hoặc lỗi mạng; thử quét lại sau.

Mặc định tắt, không bao giờ liệt kê nội dung tài khoản, và số liệu hạn mức thu
được hiện trong ngăn chi tiết phát hiện.

---

## Mức độ

- **Nghiêm trọng** — toàn quyền: chuyển tiền, chiếm tài khoản, đọc mọi thứ nó bảo
  vệ.
- **Cao** — truy cập API rộng với tư cách tài khoản.
- **Trung bình** — quyền hẹp hoặc gắn với host: một kênh, một webhook, một máy chủ
  cũng phải truy cập được.
- **Thấp** — định danh tài khoản nhưng không hành động được, ví dụ key chế độ
  test. Vẫn nên xoay khóa.

---

## Xử lý sự cố

- **“Không tới được API”** — kiểm tra địa chỉ ở thanh trên cùng và stack đang
  chạy; chấm chuyển xanh khi API trả lời.
- **Lượt quét đứng ở 0%** — mở lượt quét và xem Hoạt động trực tiếp. *Chờ giới
  hạn tốc độ của GitHub* sẽ tự qua; không có hoạt động nào nghĩa là worker không
  chạy.
- **Quét lại không thấy gì mới** — các phát hiện thuộc về lượt quét trước; hãy lọc
  trang Phát hiện theo kho.
- **Chưa rõ commit** — quét lại kho để gắn commit.
- **Nút Gửi thông báo bị mờ** — bật gửi mail thông báo trong Cài đặt.
- **Không có nút Hiện trong ngăn chi tiết** — tính năng che đang bật, owner không
  có trong danh sách bỏ che, máy chủ không có `SECRET_SEAL_KEY`, hoặc kho chưa được
  quét lại sau khi thêm owner.
- **Danh sách báo chỉ lọc trên 500 phát hiện** — thu hẹp bộ lọc phía máy chủ (mức
  độ, nhà cung cấp, trạng thái, kho) để cửa sổ bao trọn.
- **Kết quả cũ biến mất** — xóa job hoặc dọn dữ liệu sẽ xóa các phát hiện của nó;
  quét lại để có lại.

GitScanner chỉ dành cho kiểm thử bảo mật được ủy quyền. Chỉ quét những gì bạn sở
hữu hoặc được phép kiểm thử, và báo cho chủ sở hữu về những gì bạn tìm thấy.
