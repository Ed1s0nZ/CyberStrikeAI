import argparse
import json
import os.path
import re
from time import strftime, gmtime

class PrettyOuput():

    def d2b(self, a):
        tbin = []
        while a:
            tbin.append(a % 2)
            a //= 2

        t2bin = tbin[::-1]
        if len(t2bin) != 8:
            for x in range(6 - len(t2bin)):
                t2bin.insert(0, 0)
        return ''.join([str(g) for g in t2bin])

    def convert(self, time):
        if isinstance(time, str):
            return time
        if time == 0:
            return "None"
        if time == -9223372036854775808:
            return "Not Set"
        sec = abs(time) // 10000000
        days = sec // 86400
        sec -= 86400*days
        hrs = sec // 3600
        sec -= 3600*hrs
        mins = sec // 60
        sec -= 60*mins
        result = ""
        if days > 1:
            result += "{0} days ".format(days)
        elif days == 1:
            result += "{0} day ".format(days)
        if hrs > 1:
            result += "{0} hours ".format(hrs)
        elif hrs == 1:
            result += "{0} hour ".format(hrs)
        if mins > 1:
            result += "{0} minutes ".format(mins)
        elif mins == 1:
            result += "{0} minute ".format(mins)
        return result

    def password_complexity(self, data):

        print('''
    +-----------------------------------------+
    | Password Policy Information             |
    +-----------------------------------------+
    ''')

        print("[+] Password Info for Domain:", data[0]['attributes']['dc'][0].upper())
        print("\t[+] Minimum password length: ", data[0]['attributes']['instanceType'][0])
        print("\t[+] Password history length:", data[0]['attributes']['pwdHistoryLength'][0])

        password_properties = self.d2b(data[0]['attributes']['pwdProperties'][0])
        print("\t[+] Password Complexity Flags:", password_properties)
        print("")
        print("\t\t[+] Domain Refuse Password Change:", password_properties[0])
        print("\t\t[+] Domain Password Store Cleartext:", password_properties[1])
        print("\t\t[+] Domain Password Lockout Admins:", password_properties[2])
        print("\t\t[+] Domain Password No Clear Change:", password_properties[3])
        print("\t\t[+] Domain Password No Anon Change:", password_properties[4])
        print("\t\t[+] Domain Password Complex:", password_properties[5])
        print("")
        print("\t[+] Maximum password age:", self.convert(data[0]['attributes']['maxPwdAge'][0]))
        print("\t[+] Minimum password age:", self.convert(data[0]['attributes']['minPwdAge'][0]))
        print("\t[+] Reset Account Lockout Counter:", self.convert(data[0]['attributes']['lockoutDuration'][0]))
        print("\t[+] Account Lockout Threshold:", data[0]['attributes']['lockoutThreshold'][0])
        print("\t[+] Forced Log off Time:", self.convert(data[0]['attributes']['forceLogoff'][0]))

    def domain_info(self, data):
        print('''
    +--------------------------------------+
    | Getting Domain Sid For               |
    +--------------------------------------+
    ''')
        print('[+] Domain Name:', data[0]['attributes']['dc'][0])
        print('Domain Sid:', data[0]['attributes']['objectSid'][0])
        print('')
        return data[0]['attributes']['dc'][0]

    def user_info(self, users, dc):
        print('''
    +------------------------+
    | Users Infos            |
    +------------------------+
    ''')
        for user in users:
            desc = user['attributes'].get('description')[0] if user['attributes'].get('description') else "(null)"
            print("Account: " + dc + "\\" + user['attributes']['sAMAccountName'][0] + "\tName: " + user['attributes']['name'][0] + "\tDesc: " + desc)

        print("")
        for user in users:
            print("user:[" + user['attributes']['sAMAccountName'][0] + "]")
        print("")

    def groups_info(self, groups, dc):
        print('''
    +------------------------+import os.path
    | Groups Infos           |
    +------------------------+
    ''')
        for group in groups:
            print("group:[" + group['attributes']['name'][0] + "]")

        for group in groups:
            if group['attributes'].get('member'):
                users = re.findall(r"^CN=([\w\s\-\_\{\}\.\$\#]+)", '\n'.join(group['attributes']['member']), re.M)
                if users:
                    print("\n[+] Getting domain group memberships:")
                    for user in users:
                        if user == "S-1-5-11":
                            user = "NT AUTHORITY\\Authenticated Users"
                        elif user == "S-1-5-4":
                            user = "NT AUTHORITY\\INTERACTIVE"
                        elif user == "S-1-5-17":
                            user = "NT AUTHORITY\\IUSR"
                        print("Group '" + group['attributes']['name'][0] + "' has member: " + dc + "\\" + user)

def main():
    parser = argparse.ArgumentParser(description='LDAPDomainDump to pretty output like enum4linux.')

    #Main parameters
    parser.add_argument("-d", "--directory", help="The ldapdomaindump directory where the json files are saved. Required files: domain_users.json, domain_groups.json and domain_policy.json")
    args = parser.parse_args()

    if args.directory:
        with open(args.directory + '/domain_policy.json') as f:
            domain = json.load(f)
        with open(args.directory + '/domain_users.json') as f:
            users = json.load(f)
        with open(args.directory + '/domain_groups.json') as f:
            groups = json.load(f)
        pretty = PrettyOuput()
        dc = pretty.domain_info(domain)
        pretty.password_complexity(domain)
        pretty.user_info(users, dc.upper())
        pretty.groups_info(groups, dc.upper())
    else:
        print("Missing parameter --directory /output/")

if __name__ == "__main__":
    main()
