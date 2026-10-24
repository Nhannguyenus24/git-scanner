# Hướng dẫn dùng GitScanner

GitScanner clone toàn bộ lịch sử của một kho mã, duyệt từng blob duy nhất và báo
cáo các credential tìm được — ở cây hiện tại lẫn bất kỳ đâu trong quá khứ. Trang
này là hướng dẫn nhanh cho bảng điều khiển.

## Kết nối

Địa chỉ API nằm ở thanh trên; chấm tròn bên cạnh chuyển xanh khi bảng điều khiển
tới được API. Trỏ nó về host API của bạn nếu không chạy mặc định
`http://localhost:8080`. Địa chỉ, giao diện và ngôn ngữ được ghi nhớ trong trình
duyệt này.

Dùng các nút ở góc trên phải để đổi giữa **nền giấy sáng và tối** và giữa
**English và Tiếng Việt**.

## Bắt đầu một lượt quét

Bấm **Quét mới** (góc trên trái, hoặc thanh bên). Chọn đối tượng:

- **Kho** — một URL clone HTTPS, ví dụ `https://github.com/owner/repo`.
- **Người dùng** hoặc **Tổ chức** — mọi kho tài khoản đó sở hữu. Bật *gồm cả kho
  fork* để quét luôn các kho fork.

Lượt quét chạy nền. Trang mở sang màn hình chi tiết, nơi thanh tiến độ và các bộ
đếm cập nhật khi từng kho được duyệt.

## Đọc kết quả

**Tổng quan** là bức tranh chính: số phát hiện đang mở, bao nhiêu nghiêm trọng,
bao nhiêu còn sống ở HEAD, cùng các phân rã theo mức độ, nhà cung cấp và trạng
thái phân loại. Mỗi ô và mỗi thanh đều là liên kết vào danh sách Phát hiện đã
lọc sẵn.

**Phân tích** đi sâu hơn — dòng thời gian credential được đưa vào, và phân rã
theo loại ứng dụng và luật phát hiện.

**Phát hiện** là danh sách đầy đủ, lọc được. Lọc theo mức độ, nhà cung cấp, trạng
thái hoặc từ khóa; bộ lọc nằm trên URL nên có thể chia sẻ hay lưu dấu. Mỗi dòng
hiển thị:

- mức độ và nhà cung cấp của credential;
- bản xem trước đã che — secret thật không bao giờ rời khỏi engine;
- file và dòng, kèm liên kết tới file trên GitHub;
- commit đã đưa nó vào, tác giả và ngày;
- **Mức lộ** — `ở HEAD` nghĩa là vẫn còn trong cây hiện tại; `lịch sử` nghĩa là
  đã bị xóa nhưng vẫn lấy được khi clone;
- nhãn **Còn sống — xoay khóa** khi kiểm tra xác nhận key vẫn dùng được.

Dùng menu ở cuối mỗi dòng để đánh dấu **Đã xoay khóa**, **Báo nhầm** hoặc **Bỏ
qua** khi xử lý.

## Kho mã và dạng chưa gắn

**Kho mã** xếp hạng mọi kho đã quét theo mức độ lộ đang mở, chia theo mức độ.
**Chưa gắn** là tồn đọng các bí mật entropy cao chưa luật nào nhận, gom theo hình
dạng — dạng xuất hiện nhiều là nhà cung cấp đáng viết luật.

## Kiểm tra còn sống

Khi người vận hành bật cho các mục tiêu của chính họ, GitScanner gọi một lời
nhận diện chỉ-đọc cho mỗi phát hiện để xác nhận key còn dùng được không và thuộc
tài khoản nào. Key còn sống là thứ cần xoay khóa trước tiên. Mặc định tắt và
không bao giờ liệt kê nội dung tài khoản.
