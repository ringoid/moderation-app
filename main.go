package main

import (
	"net/http"
	"html/template"
	"./apimodel"
	"fmt"
	"time"
	"strings"
	"encoding/json"
	"sort"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/ringoid/commons"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
)

//todo:make custom
var env = "prod"
var photoSize = "480x640"

const (
	BLOCK_ACTION = "block"

	HIDE_ACTION    = "hide"
	NOTHING_ACTION = "nothing"

	BLOCK_PHOTO_AND_NOTIFY_USER = "blockPhoto"
	HIDE_PHOTO_SHOW_ONLY_OWNER  = "hidePhoto"
	HIDE_PROFILE_DONT_TELL      = "hideProfile"
	COMPLETE_WITH_IT            = "complete"

	COLUMN_IN_ONE_ROW = 4
)

var currentReportedProfiles []apimodel.ProfileObj

var AwsLambdaClient *lambda.Lambda
var ModerationFunctionName string

func init() {
	ModerationFunctionName = fmt.Sprintf("%s-%s", env, "moderation-relationships")
	awsSession, err := session.NewSession(aws.NewConfig().WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries))
	if err != nil {
		panic(err)
	}
	AwsLambdaClient = lambda.New(awsSession)
}

func main() {
	server := http.Server{
		Addr: "127.0.0.1:8080",
	}

	http.HandleFunc("/", indexScreen)
	http.HandleFunc("/home", indexScreen)
	http.HandleFunc("/reported", reportedScreen)
	http.HandleFunc("/justnew", justNewScreen)

	server.ListenAndServe()
}

func profilesPage(writer http.ResponseWriter, request *http.Request, moderationRequest apimodel.ModerationReq, submitAction string) {
	if request.Method != http.MethodPost {
		data := getProfilesWithPhoto(moderationRequest)

		currentReportedProfiles = data.Profiles

		data.SubmitAction = submitAction
		if len(data.Profiles) != 0 {
			temp := template.Must(template.ParseFiles("templates/layout.html", "templates/content.html"))
			temp.ExecuteTemplate(writer, "layout", data)
			return
		} else {
			data := apimodel.ReportedFormData{
				ShowData: false,
				Message:  fmt.Sprintf("no new data at %v", time.Now().Local()),
			}
			temp := template.Must(template.ParseFiles("templates/layout.html", "templates/content.html"))
			temp.ExecuteTemplate(writer, "layout", data)
			return
		}
	}

	if currentReportedProfiles == nil {
		data := apimodel.ReportedFormData{
			ShowData: false,
			Message:  "session was empty",
		}
		temp := template.Must(template.ParseFiles("templates/layout.html", "templates/content.html"))
		temp.ExecuteTemplate(writer, "layout", data)
		return
	}

	request.ParseForm()

	requestSlice := make([]apimodel.ModerationReq, 0)
	for _, eachProfiles := range currentReportedProfiles {
		profileState := request.PostFormValue(fmt.Sprintf("%s_user_state", eachProfiles.UserId))
		if profileState == HIDE_ACTION {
			requestSlice = append(requestSlice, apimodel.ModerationReq{
				QueryType: HIDE_PROFILE_DONT_TELL,
				ProfilePhotoMap: map[string]string{
					eachProfiles.UserId: "",
				},
			})
		}
		for _, eachPhoto := range eachProfiles.Photos {
			photoState := request.PostFormValue(fmt.Sprintf("%s_photo_state", eachPhoto.PhotoId))
			if len(photoState) == 0 {
				continue
			}
			switch photoState {
			case BLOCK_ACTION:
				requestSlice = append(requestSlice, apimodel.ModerationReq{
					QueryType: BLOCK_PHOTO_AND_NOTIFY_USER,
					ProfilePhotoMap: map[string]string{
						eachProfiles.UserId: eachPhoto.PhotoId,
					},
				})
			case HIDE_ACTION:
				requestSlice = append(requestSlice, apimodel.ModerationReq{
					QueryType: HIDE_PHOTO_SHOW_ONLY_OWNER,
					ProfilePhotoMap: map[string]string{
						eachProfiles.UserId: eachPhoto.PhotoId,
					},
				})
			case NOTHING_ACTION:
				//don't do anything
			default:
				panic(fmt.Sprintf("unsupported action type [%s]", photoState))
			}
			requestSlice = append(requestSlice, apimodel.ModerationReq{
				QueryType: COMPLETE_WITH_IT,
				ProfilePhotoMap: map[string]string{
					eachProfiles.UserId: eachPhoto.PhotoId,
				},
			})
		} //end iterate by photos
	} //end iterate by profiles

	for _, req := range requestSlice {
		moderationFunctionCall(req)
	}

	currentReportedProfiles = nil

	data := apimodel.ReportedFormData{
		ShowData: false,
		Message:  "done",
	}
	temp := template.Must(template.ParseFiles("templates/layout.html", "templates/content.html"))
	temp.ExecuteTemplate(writer, "layout", data)

}

func indexScreen(writer http.ResponseWriter, request *http.Request) {
	temp := template.Must(template.ParseFiles("templates/layout.html", "templates/hello.html"))
	temp.ExecuteTemplate(writer, "layout", "Hello, world!")
}

func reportedScreen(writer http.ResponseWriter, request *http.Request) {
	reportedRequest := apimodel.ModerationReq{
		QueryType: "reported",
		Limit:     10,
	}
	profilesPage(writer, request, reportedRequest, "/reported")
}

func justNewScreen(writer http.ResponseWriter, request *http.Request) {
	reportedRequest := apimodel.ModerationReq{
		QueryType: "unReported",
		Limit:     10,
	}
	profilesPage(writer, request, reportedRequest, "/justnew")
}

func getProfilesWithPhoto(request apimodel.ModerationReq) apimodel.ReportedFormData {
	payload := moderationFunctionCall(request)
	var resp apimodel.ModerationResp
	err := json.Unmarshal(payload, &resp)
	if err != nil {
		panic(err)
	}
	//make url for each photo
	for profileIndex, eachProfile := range resp.Profiles {
		for photoIndex := range eachProfile.Photos {
			photoObj := resp.Profiles[profileIndex].Photos[photoIndex]
			resizedPhoto := fmt.Sprintf("%s_%s.%s", strings.TrimPrefix(photoObj.PhotoId, "origin_"), photoSize, "jpg")
			link := fmt.Sprintf("https://s3-eu-west-1.amazonaws.com/%s-ringoid-public-photo/%s", env, resizedPhoto)
			//fmt.Println(link)
			resp.Profiles[profileIndex].Photos[photoIndex].PhotoUrl = link
		}
	}

	//sort photo in a response
	for profileIndex, _ := range resp.Profiles {
		eachProfile := resp.Profiles[profileIndex]
		photos := eachProfile.Photos
		sort.Slice(photos, func(one, two int) bool {
			photoOne := photos[one]
			photoTwo := photos[two]

			var oneCount int
			var twoCount int

			if photoOne.PhotoReported {
				oneCount += 3
			}
			if photoTwo.PhotoReported {
				twoCount += 3
			}

			if !photoOne.WasModeratedBefore {
				oneCount += 2
			} else {
				oneCount -= 5
			}
			if !photoTwo.WasModeratedBefore {
				twoCount += 2
			} else {
				twoCount -= 5
			}

			return twoCount < oneCount
		})
		//delete duplicates from block reasons
		for photoIndex, _ := range photos {
			blockReasons := photos[photoIndex].BlockReasons
			reasonMap := make(map[int]bool)
			for _, reason := range blockReasons {
				reasonMap[reason] = true
			}
			finalReasons := make([]int, 0)
			for key, value := range reasonMap {
				if value {
					finalReasons = append(finalReasons, key)
				}
			}
			photos[photoIndex].BlockReasons = finalReasons
		}
	}

	for profileIndex, _ := range resp.Profiles {
		eachProfileObj := resp.Profiles[profileIndex]

		rows := make([]apimodel.Row, 0)
		row := apimodel.Row{
			Photos: make([]apimodel.PhotoObj, 0),
		}

		var howManyPhotosWereBlocked int

		var columnCount int
		for _, eachPhotoObj := range eachProfileObj.Photos {
			//don't show blocked photo but count them
			if eachPhotoObj.PhotoHidden {
				howManyPhotosWereBlocked++
				continue
			}

			row.Photos = append(row.Photos, eachPhotoObj)
			columnCount++
			if columnCount == COLUMN_IN_ONE_ROW {
				rows = append(rows, row)
				columnCount = 0
				row = apimodel.Row{
					Photos: make([]apimodel.PhotoObj, 0),
				}
			}
		}
		if len(row.Photos) > 0 {
			rows = append(rows, row)
		}

		resp.Profiles[profileIndex].Rows = rows
		resp.Profiles[profileIndex].HowManyPhotosWereBlocked = howManyPhotosWereBlocked
	}

	return apimodel.ReportedFormData{
		Profiles: resp.Profiles,
		ShowData: true,
	}
}

func moderationFunctionCall(request apimodel.ModerationReq) []byte {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}

	resp, err := AwsLambdaClient.Invoke(&lambda.InvokeInput{FunctionName: aws.String(ModerationFunctionName), Payload: jsonBody})
	if err != nil {
		panic(err)
	}

	if *resp.StatusCode != 200 {
		panic(err)
	}

	return resp.Payload
}
