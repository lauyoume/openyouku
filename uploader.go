package openyouku

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/astaxie/beego"
)

var (
	CHUNK_SIZE = int64(5 * 1024 * 1024) // 5M 分片
)

type VideoParam struct {
	Title           string `json:"title"`
	Tags            string `json:"tags"`
	Description     string `json:"description,omitempty"`
	Original        string `json:"original,omitempty"`
	CategoryName    string `json:"category_name,omitempty"`
	SubcategoryIds  string `json:"subcategory_ids,omitempty"`
	ThumbnailCustom string `json:"thumbnail_custom,omitempty"`
}

type Uploader struct {
	content  []byte
	fileName string
	apiParam map[string]string
	sdk      *SDK
}

func NewUploader(sdk *SDK, name string, content []byte) *Uploader {
	uploader := new(Uploader)
	uploader.content = content
	uploader.fileName = name
	uploader.apiParam = make(map[string]string, 0)
	uploader.Set("file_name", name)
	uploader.Set("file_size", fmt.Sprint(len(uploader.content)))
	uploader.Set("file_md5", md5bytes(uploader.content))
	uploader.sdk = sdk
	return uploader
}

func NewUploaderWithFile(sdk *SDK, filename string) (*Uploader, error) {
	uploader := new(Uploader)
	uploader.fileName = filename
	uploader.apiParam = make(map[string]string, 0)
	uploader.Set("file_name", filename)
	uploader.sdk = sdk

	fi, err := os.Stat(uploader.fileName)
	if err != nil {
		return nil, err
	}
	vmd5, err := getmd5(uploader.fileName)
	if err != nil {
		return nil, err
	}
	uploader.Set("file_size", fmt.Sprint(fi.Size()))
	uploader.Set("file_md5", vmd5)
	return uploader, nil
}

func (uploader *Uploader) Set(k, v string) {
	uploader.apiParam[k] = v
}

//获取上传权限
func (uploader *Uploader) getUploadToken() (*UploadResponse, error) {
	rand.Seed(int64(time.Now().UTC().Nanosecond()))
	ip := fmt.Sprintf("%d.%d.%d.%d", 1+rand.Intn(200), 1+rand.Intn(100), 50+rand.Intn(50), 1+rand.Intn(200))
	if uploader.content == nil {
	}
	uploader.Set("client_ip", ip)
	uploader.Set("server_type", "oupload")
	body, err := uploader.sdk.PostBytes("youku.video.upload.create", uploader.apiParam)
	if err != nil {
		return nil, err
	}
	resp := &UploadResponse{}
	err = json.Unmarshal(body, resp)
	return resp, err
}

//鉴权失败重新获取
func (uploader *Uploader) refreshBucket(p *OssParam) (*oss.Bucket, error) {
	params := map[string]string{
		"upload_token": p.UploadToken,
		"oss_bucket":   p.OssBucket,
		"oss_object":   p.OssObject,
	}
	body, err := uploader.sdk.PostBytes("youku.video.upload.getsts", params)
	if err != nil {
		return nil, err
	}
	resp := &UploadResponse{}
	err = json.Unmarshal(body, resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) < 1 {
		return nil, errors.New("Can not get oss upload param")
	}

	return uploader.getBucket(&resp.Data[0])
}

//开始上传
func (uploader *Uploader) doUpload(p *OssParam, part bool) error {
	bucket, err := uploader.getBucket(p)
	if err != nil {
		return err
	}

	signed_url, err := bucket.SignURL(p.OssObject, oss.HTTPPut, 3600)
	if err != nil {
		return err
	}
	if uploader.content != nil {
		return bucket.PutObjectWithURL(signed_url, bytes.NewReader(uploader.content))
	} else {
		if !part {
			return bucket.PutObjectFromFileWithURL(signed_url, uploader.fileName)
		}

		chunks, err := oss.SplitFileByPartSize(uploader.fileName, CHUNK_SIZE)
		if err != nil {
			return errors.New("Split Part Error: " + err.Error())
		}
		imur, err := bucket.InitiateMultipartUpload(p.OssObject)
		if err != nil {
			return errors.New("Init Part Error: " + err.Error())
		}

		var parts []oss.UploadPart
		fd, err := os.Open(uploader.fileName)
		if err != nil {
			return err
		}
		defer fd.Close()
		for _, chunk := range chunks {
			// 对每个分片调用UploadPart方法上传。
			has := false
			for i := 0; i < 15; i++ {
				fd.Seek(chunk.Offset, os.SEEK_SET)
				part, err := bucket.UploadPart(imur, fd, chunk.Size, chunk.Number)
				if err != nil {
					beego.Error(fmt.Sprintf("[chunk %d] %s, UploadPart Error:", chunk.Number, uploader.fileName), err)
					//鉴权过期,获取新的鉴权
					if strings.Contains(err.Error(), "InvalidAccessKeyId") {
						bucket, err = uploader.refreshBucket(p)
						if err != nil {
							return err
						}
					}
					continue
				}
				has = true
				//beego.Debug(fmt.Sprintf("[%d] - chunk size %d", chunk.Number, chunk.Size))
				parts = append(parts, part)
				break
			}
			if !has {
				// 取消分片上传
				bucket.AbortMultipartUpload(imur)
				text := fmt.Sprintf("%s [chunk %d] upload try max time", uploader.fileName, chunk.Number)
				return errors.New(text)
			}
		}
		// 步骤3：完成分片上传
		cmur, err := bucket.CompleteMultipartUpload(imur, parts)
		if err != nil {
			beego.Error("CompleteMultipartUpload Error:", err)
			return err
		}
		beego.Debug("cmur:", cmur)
		return nil
	}
}

func (uploader *Uploader) getBucket(p *OssParam) (*oss.Bucket, error) {
	client, err := oss.New(p.Endpoint, p.TempAccessID, p.TempAccessSecret, oss.SecurityToken(p.SecurityToken))
	// client, err := oss.New("oss-accelerate.aliyuncs.com", p.TempAccessID, p.TempAccessSecret, oss.SecurityToken(p.SecurityToken))
	if err != nil {
		return nil, err
	}
	client.Config.HTTPTimeout.HeaderTimeout = 20 * time.Second

	return client.Bucket(p.OssBucket)
}

func (uploader *Uploader) Upload(part bool, custom *OssParam) (*VideoData, error) {
	// 获取oss上传信息
	upresp, err := uploader.getUploadToken()
	if err != nil {
		return nil, err
	}

	if upresp.Error.ErrorCode < 0 {
		return nil, errors.New(upresp.Error.ErrorMsg)
	}

	if len(upresp.Data) < 1 {
		return nil, errors.New("Can not get oss upload param")
	}

	param := upresp.Data[0]

	if custom != nil {
		// 临时修改美国bucket
		param.Endpoint = custom.Endpoint
		param.OssBucket = custom.OssBucket
		param.TempAccessID = custom.TempAccessID
		param.TempAccessSecret = custom.TempAccessSecret
		param.SecurityToken = custom.SecurityToken
	}
	beego.Info(param)

	// oss上传视频
	err = uploader.doUpload(&param, part)
	if err != nil {
		return nil, err
	}

	data := &VideoData{
		Vid:         upresp.Data[0].Vid,
		UploadToken: upresp.Data[0].UploadToken,
	}

	if custom != nil {
		data.OwOssBucket = custom.OssBucket
		data.OwOssObject = param.OssObject
	}

	return data, nil
}

func (sdk *SDK) doSave(video *VideoParam, data *VideoData) (*VideoResponse, error) {
	video_json, _ := json.Marshal(video)
	params := make(map[string]string)

	if err := json.Unmarshal(video_json, &params); err != nil {
		return nil, err
	}

	params["upload_token"] = data.UploadToken
	body, err := sdk.PostBytes("youku.video.upload.save", params)
	if err != nil {
		return nil, err
	}
	resp := &VideoResponse{}
	err = json.Unmarshal(body, resp)
	return resp, err
}

func (sdk *SDK) doComplete(data *VideoData) (*VideoResponse, error) {
	params := make(map[string]string)
	params["upload_token"] = data.UploadToken
	if data.OwOssBucket != "" {
		params["ow_oss_bucket"] = data.OwOssBucket
	}
	if data.OwOssObject != "" {
		params["ow_oss_object"] = data.OwOssObject
	}
	body, err := sdk.PostBytes("youku.video.upload.complete", params)
	if err != nil {
		return nil, err
	}
	base := Response{}
	if err := json.Unmarshal(body, &base); err != nil {
		return nil, err
	}
	if base.Error.ErrorCode < 0 {
		return nil, errors.New(base.Error.ErrorMsg)
	}
	resp := &VideoResponse{}
	err = json.Unmarshal(body, resp)
	return resp, err
}

func (sdk *SDK) SaveVideo(video *VideoParam, data *VideoData) (*VideoData, error) {
	// 保存视频信息
	vresp, err := sdk.doSave(video, data)
	if err != nil {
		return nil, errors.New("doSave error, " + err.Error())
	}
	if len(vresp.Data) < 1 {
		return nil, errors.New("doSave error, vresp.Data empty")
	}

	// 关键一步尝试重试预防网络错误
	for i := 0; i < 5; i++ {
		// 完成上传任务
		vresp, err = sdk.doComplete(data)
		if err != nil {
			beego.Error("complete video error: ", err)
			continue
		}

		if len(vresp.Data) < 1 {
			beego.Error("complete video error: ", err)
			continue
		}
		break
	}

	if err != nil {
		return nil, errors.New("doComplete error, " + err.Error())
	}

	rdata := vresp.Data[0]
	return &rdata, nil
}

func (uploader *Uploader) UploadAndSave(video *VideoParam, part bool) (*VideoData, error) {
	// 获取oss上传信息
	upresp, err := uploader.getUploadToken()
	if err != nil {
		return nil, err
	}

	if upresp.Error.ErrorCode < 0 {
		return nil, errors.New(upresp.Error.ErrorMsg)
	}

	if len(upresp.Data) < 1 {
		return nil, errors.New("Can not get oss upload param")
	}
	// oss上传视频
	err = uploader.doUpload(&upresp.Data[0], part)
	if err != nil {
		return nil, err
	}

	// 保存视频信息
	data := &VideoData{
		Vid:         upresp.Data[0].Vid,
		UploadToken: upresp.Data[0].UploadToken,
	}
	return uploader.sdk.SaveVideo(video, data)
}

const filechunk = 8192 // we settle for 8KB
func getmd5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	info, _ := file.Stat()
	filesize := info.Size()
	blocks := uint64(math.Ceil(float64(filesize) / float64(filechunk)))
	hash := md5.New()
	for i := uint64(0); i < blocks; i++ {
		blocksize := int(math.Min(filechunk, float64(filesize-int64(i*filechunk))))
		buf := make([]byte, blocksize)

		file.Read(buf)
		io.WriteString(hash, string(buf)) // append into the hash
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func md5bytes(b []byte) string {
	m := md5.New()
	m.Write(b)
	return fmt.Sprintf("%x", m.Sum(nil))
}
