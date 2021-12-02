package qqwry

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// @ref https://github.com/freshcn/qqwry

const (
	// Index len
	indexLen = 7
	// RedirectMode1 国家的类型, 指向另一个指向
	redirectMode1 = 0x01
	// RedirectMode2 国家的类型, 指向一个指向
	redirectMode2 = 0x02
)

func New(filePath string) (*QQWry, error) {
	var tmpData []byte

	if _, err := os.Stat(filePath); err != nil && os.IsNotExist(err) {
		// 从网络获取最新纯真 IP 库 ，保存到本地
		tmpData, err = new(Loader).Download()
		if err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(filePath, tmpData, 0644); err != nil {
			return nil, err
		}
	} else {
		dist, err := os.OpenFile(filePath, os.O_RDONLY, 0400)
		if err != nil {
			return nil, err
		}
		defer dist.Close()

		tmpData, err = ioutil.ReadAll(dist)
		if err != nil {
			return nil, err
		}
	}
	// buf := tmpData[0:8]
	// start := binary.LittleEndian.Uint32(buf[:4])
	// end := binary.LittleEndian.Uint32(buf[4:])
	// ipNum := int64((end-start)/IndexLen + 1)

	return &QQWry{data: tmpData}, nil
}

type result struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Area    string `json:"area"`
}

type QQWry struct {
	data   []byte
	offset int64
}

func (q *QQWry) Find(ip string) (*result, error) {
	if strings.Count(ip, ".") != 3 {
		return nil, fmt.Errorf("")
	}
	offset := q.searchIndex(binary.BigEndian.Uint32(net.ParseIP(ip).To4()))
	if offset <= 0 {
		return nil, fmt.Errorf("")
	}

	var country []byte
	var area []byte

	mode := q.readMode(offset + 4)
	if mode == redirectMode1 {
		countryOffset := q.readUInt24()
		mode = q.readMode(countryOffset)
		if mode == redirectMode2 {
			c := q.readUInt24()
			country = q.readString(c)
			countryOffset += 4
		} else {
			country = q.readString(countryOffset)
			countryOffset += uint32(len(country) + 1)
		}
		area = q.readArea(countryOffset)
	} else if mode == redirectMode2 {
		countryOffset := q.readUInt24()
		country = q.readString(countryOffset)
		area = q.readArea(offset + 8)
	} else {
		country = q.readString(offset + 4)
		area = q.readArea(offset + uint32(5+len(country)))
	}

	enc := simplifiedchinese.GBK.NewDecoder()

	res := result{}
	res.IP = ip
	res.Country, _ = enc.String(string(country))
	res.Area, _ = enc.String(string(area))
	if strings.ToUpper(strings.TrimSpace(res.Area)) == "CZ88.NET" {
		res.Area = ""
	}

	return &res, nil
}

func (q *QQWry) readData(num int, offset ...int64) (rs []byte) {
	if len(offset) > 0 {
		q.offset = offset[0]
	}
	nums := int64(num)
	end := q.offset + nums
	dataNum := int64(len(q.data))
	if q.offset > dataNum {
		return nil
	}

	if end > dataNum {
		end = dataNum
	}
	rs = q.data[q.offset:end]
	q.offset = end
	return
}

func (q *QQWry) readMode(offset uint32) byte {
	mode := q.readData(1, int64(offset))
	return mode[0]
}

func (q *QQWry) readArea(offset uint32) []byte {
	mode := q.readMode(offset)
	if mode == redirectMode1 || mode == redirectMode2 {
		areaOffset := q.readUInt24()
		if areaOffset == 0 {
			return []byte("")
		}
		return q.readString(areaOffset)
	}
	return q.readString(offset)
}

func (q *QQWry) readString(offset uint32) []byte {
	q.offset = int64(offset)
	data := make([]byte, 0, 30)
	buf := make([]byte, 1)
	for {
		buf = q.readData(1)
		if buf[0] == 0 {
			break
		}
		data = append(data, buf[0])
	}
	return data
}

func (q *QQWry) searchIndex(ip uint32) uint32 {
	header := q.readData(8, 0)

	start := binary.LittleEndian.Uint32(header[:4])
	end := binary.LittleEndian.Uint32(header[4:])

	buf := make([]byte, indexLen)
	mid := uint32(0)
	_ip := uint32(0)

	for {
		mid = q.getMiddleOffset(start, end)
		buf = q.readData(indexLen, int64(mid))
		_ip = binary.LittleEndian.Uint32(buf[:4])

		if end-start == indexLen {
			offset := q.byteToUInt32(buf[4:])
			buf = q.readData(indexLen)
			if ip < binary.LittleEndian.Uint32(buf[:4]) {
				return offset
			}
			return 0
		}

		if _ip > ip {
			end = mid
		} else if _ip < ip {
			start = mid
		} else if _ip == ip {
			return q.byteToUInt32(buf[4:])
		}
	}
}

func (q *QQWry) readUInt24() uint32 {
	buf := q.readData(3)
	return q.byteToUInt32(buf)
}

func (q *QQWry) getMiddleOffset(start uint32, end uint32) uint32 {
	records := ((end - start) / indexLen) >> 1
	return start + records*indexLen
}

func (q *QQWry) byteToUInt32(data []byte) uint32 {
	i := uint32(data[0]) & 0xff
	i |= (uint32(data[1]) << 8) & 0xff00
	i |= (uint32(data[2]) << 16) & 0xff0000
	return i
}
